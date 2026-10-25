package owner

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo is the PostgreSQL data access layer for owner features.
type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

func (r *Repo) OwnedVenueIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT id FROM venues WHERE owner_id = $1 ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("select owned venues: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("scan owned venues: %w", err)
	}
	return ids, nil
}

// VenueOwner returns the venue's owner_id (0 when it has none).
func (r *Repo) VenueOwner(ctx context.Context, venueID int64) (int64, error) {
	var ownerID int64
	err := r.db.QueryRow(ctx, `SELECT COALESCE(owner_id, 0) FROM venues WHERE id = $1`, venueID).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("select venue owner: %w", err)
	}
	return ownerID, nil
}

// BookingVenueOwner returns the booking's venue and that venue's owner_id (0 when none).
func (r *Repo) BookingVenueOwner(ctx context.Context, bookingID int64) (venueID, ownerID int64, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT v.id, COALESCE(v.owner_id, 0)
		FROM bookings b JOIN venues v ON v.id = b.venue_id
		WHERE b.id = $1`, bookingID).Scan(&venueID, &ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, ErrNotFound
	}
	if err != nil {
		return 0, 0, fmt.Errorf("select booking venue owner: %w", err)
	}
	return venueID, ownerID, nil
}
