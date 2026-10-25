package venue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/TableBot/backend/internal/bookingstatus"
)

// Repo is the PostgreSQL data access layer for venues and tables.
type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

const venueColumns = `v.id, v.name, v.category, v.address, v.description, v.timezone,
	v.opens_at, v.closes_at, v.booking_minutes, v.slot_step_minutes, v.floor_plan_url`

func scanVenue(row pgx.Row, extra ...any) (Venue, error) {
	var v Venue
	var opens, closes pgtype.Time
	dest := append([]any{&v.ID, &v.Name, &v.Category, &v.Address, &v.Description, &v.Timezone,
		&opens, &closes, &v.BookingMinutes, &v.SlotStepMinutes, &v.FloorPlanURL}, extra...)
	if err := row.Scan(dest...); err != nil {
		return Venue{}, err
	}
	// TIME is microseconds since midnight; 24:00 is 1440 minutes.
	v.OpensMinute = int(opens.Microseconds / int64(time.Minute/time.Microsecond))
	v.ClosesMinute = int(closes.Microseconds / int64(time.Minute/time.Microsecond))
	v.OpensAt, v.ClosesAt = clock(v.OpensMinute), clock(v.ClosesMinute)
	return v, nil
}

func (r *Repo) List(ctx context.Context, category string) ([]Venue, error) {
	rows, err := r.db.Query(ctx, `SELECT `+venueColumns+` FROM venues v
		WHERE v.is_active AND ($1 = '' OR v.category = $1) ORDER BY v.id`, category)
	if err != nil {
		return nil, fmt.Errorf("select venues: %w", err)
	}
	defer rows.Close()
	out := []Venue{}
	for rows.Next() {
		v, err := scanVenue(rows)
		if err != nil {
			return nil, fmt.Errorf("scan venue: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Get returns an active venue or ErrNotFound.
func (r *Repo) Get(ctx context.Context, id int64) (Venue, error) {
	v, err := scanVenue(r.db.QueryRow(ctx, `SELECT `+venueColumns+` FROM venues v WHERE v.id = $1 AND v.is_active`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Venue{}, ErrNotFound
	}
	if err != nil {
		return Venue{}, fmt.Errorf("select venue: %w", err)
	}
	return v, nil
}

// TableWithVenue returns an active table of an active venue, or ErrNotFound.
func (r *Repo) TableWithVenue(ctx context.Context, tableID int64) (Venue, Table, error) {
	var t Table
	v, err := scanVenue(r.db.QueryRow(ctx, `
		SELECT `+venueColumns+`, t.id, t.label, t.seats, t.x::float8, t.y::float8, t.shape
		FROM venue_tables t JOIN venues v ON v.id = t.venue_id
		WHERE t.id = $1 AND t.is_active AND v.is_active`, tableID),
		&t.ID, &t.Label, &t.Seats, &t.X, &t.Y, &t.Shape)
	if errors.Is(err, pgx.ErrNoRows) {
		return Venue{}, Table{}, ErrNotFound
	}
	if err != nil {
		return Venue{}, Table{}, fmt.Errorf("select table: %w", err)
	}
	return v, t, nil
}

// Tables returns the venue's active tables.
func (r *Repo) Tables(ctx context.Context, venueID int64) ([]Table, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, label, seats, x::float8, y::float8, shape
		FROM venue_tables WHERE venue_id = $1 AND is_active ORDER BY id`, venueID)
	if err != nil {
		return nil, fmt.Errorf("select tables: %w", err)
	}
	tables, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Table, error) {
		var t Table
		err := row.Scan(&t.ID, &t.Label, &t.Seats, &t.X, &t.Y, &t.Shape)
		return t, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan tables: %w", err)
	}
	return tables, nil
}

// span is a blocking booking's table and [start, end).
type span struct {
	tableID    int64
	start, end time.Time
}

func (s span) overlaps(start, end time.Time) bool {
	return s.start.Before(end) && start.Before(s.end)
}

// blockingSpans loads, in one query, the venue's blocking bookings overlapping [from, to).
func (r *Repo) blockingSpans(ctx context.Context, venueID int64, from, to time.Time) ([]span, error) {
	rows, err := r.db.Query(ctx, `
		SELECT b.table_id, lower(b.during), upper(b.during)
		FROM bookings b
		WHERE b.venue_id = $1 AND b.during && tstzrange($2, $3, '[)') AND `+bookingstatus.BlockingSQL,
		venueID, from, to)
	if err != nil {
		return nil, fmt.Errorf("select blocking bookings: %w", err)
	}
	spans, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (span, error) {
		var s span
		err := row.Scan(&s.tableID, &s.start, &s.end)
		return s, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan blocking bookings: %w", err)
	}
	return spans, nil
}
