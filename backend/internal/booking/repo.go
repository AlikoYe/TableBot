package booking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/TableBot/backend/internal/bookingstatus"
	"github.com/AlikoYe/TableBot/backend/internal/venue"
)

// Repo is the PostgreSQL data access layer for bookings.
type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

type insertRow struct {
	VenueID, TableID, UserID int64
	Guests                   int
	StartsAt                 time.Time
	BookingMinutes           int
	HoldMinutes              int
	SpecialRequest           string
}

// Insert expires overdue holds on the table or user, then inserts a pending booking, in one transaction.
func (r *Repo) Insert(ctx context.Context, in insertRow) (int64, error) {
	var id int64
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		// Lock table then user (always this order) so racing inserts queue instead of deadlocking
		// inside the exclusion constraints; the loser then gets a clean 23P01.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM venue_tables WHERE id = $1 FOR NO KEY UPDATE`, in.TableID); err != nil {
			return fmt.Errorf("lock table: %w", err)
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE telegram_id = $1 FOR NO KEY UPDATE`, in.UserID); err != nil {
			return fmt.Errorf("lock user: %w", err)
		}
		if _, err := tx.Exec(ctx, bookingstatus.ExpireOverdueSQL, in.TableID, in.UserID); err != nil {
			return fmt.Errorf("expire overdue holds: %w", err)
		}
		return tx.QueryRow(ctx, `
			INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status, expires_at, special_request)
			VALUES ($1, $2, $3, $4, tstzrange($5::timestamptz, $5::timestamptz + make_interval(mins => $6), '[)'), 'pending',
			        now() + make_interval(mins => $7), $8)
			RETURNING id`,
			in.VenueID, in.TableID, in.UserID, in.Guests, in.StartsAt, in.BookingMinutes, in.HoldMinutes, in.SpecialRequest,
		).Scan(&id)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
		switch pgErr.ConstraintName {
		case "bookings_no_table_overlap":
			return 0, ErrTableTaken
		case "bookings_no_user_overlap":
			return 0, ErrUserBusy
		}
	}
	if err != nil {
		return 0, fmt.Errorf("insert booking: %w", err)
	}
	return id, nil
}

const selectBooking = `
	SELECT b.id, b.user_id, ` + bookingstatus.EffectiveSQL + `, b.guests, lower(b.during), upper(b.during),
	       b.expires_at, b.special_request, b.created_at, v.id, v.name, v.timezone, COALESCE(v.owner_id, 0),
	       t.id, t.label, t.seats, u.name, u.username
	FROM bookings b
	JOIN venues v ON v.id = b.venue_id
	JOIN venue_tables t ON t.id = b.table_id
	JOIN users u ON u.telegram_id = b.user_id`

func scanBooking(row pgx.Row) (Booking, error) {
	var b Booking
	var tz string
	err := row.Scan(&b.ID, &b.UserID, &b.Status, &b.Guests, &b.StartsAt, &b.EndsAt,
		&b.ExpiresAt, &b.SpecialRequest, &b.CreatedAt, &b.Venue.ID, &b.Venue.Name, &tz, &b.OwnerID,
		&b.Table.ID, &b.Table.Label, &b.Table.Seats, &b.GuestInfo.Name, &b.GuestInfo.Username)
	if err != nil {
		return Booking{}, err
	}
	loc, err := venue.Location(tz)
	if err != nil {
		return Booking{}, err
	}
	b.StartsAt, b.EndsAt = b.StartsAt.In(loc), b.EndsAt.In(loc)
	b.CreatedAt = b.CreatedAt.In(loc).Truncate(time.Second)
	if b.Status == bookingstatus.Pending && b.ExpiresAt != nil {
		t := b.ExpiresAt.In(loc).Truncate(time.Second)
		b.ExpiresAt = &t
	} else {
		b.ExpiresAt = nil
	}
	return b, nil
}

func (r *Repo) Get(ctx context.Context, id int64) (Booking, error) {
	b, err := scanBooking(r.db.QueryRow(ctx, selectBooking+` WHERE b.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrNotFound
	}
	if err != nil {
		return Booking{}, fmt.Errorf("select booking: %w", err)
	}
	return b, nil
}

// ListForVenue returns the venue's bookings starting in [from, to), ordered by start.
func (r *Repo) ListForVenue(ctx context.Context, venueID int64, from, to time.Time) ([]Booking, error) {
	return r.list(ctx, selectBooking+`
		WHERE b.venue_id = $1 AND lower(b.during) >= $2 AND lower(b.during) < $3
		ORDER BY lower(b.during), b.id`, venueID, from, to)
}

// ListForUser returns the user's newest bookings by start time, at most limit.
func (r *Repo) ListForUser(ctx context.Context, userID int64, limit int) ([]Booking, error) {
	return r.list(ctx, selectBooking+`
		WHERE b.user_id = $1 ORDER BY lower(b.during) DESC, b.id DESC LIMIT $2`, userID, limit)
}

func (r *Repo) list(ctx context.Context, sql string, args ...any) ([]Booking, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("select bookings: %w", err)
	}
	defer rows.Close()
	out := []Booking{}
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Cancel cancels the user's blocking booking that has not started yet.
func (r *Repo) Cancel(ctx context.Context, userID, bookingID int64) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE bookings b SET status = 'cancelled', updated_at = now()
		WHERE b.id = $1 AND b.user_id = $2 AND lower(b.during) > now() AND `+bookingstatus.BlockingSQL,
		bookingID, userID)
	if err != nil {
		return fmt.Errorf("cancel booking: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	var owner int64
	err = r.db.QueryRow(ctx, `SELECT user_id FROM bookings WHERE id = $1`, bookingID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != userID) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("select booking owner: %w", err)
	}
	return ErrNotCancellable
}
