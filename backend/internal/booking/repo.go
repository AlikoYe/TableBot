package booking

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo is the PostgreSQL data access layer for bookings.
// TODO: replace the mock data below with queries against bookings/time_slots.
type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(ctx context.Context, in CreateInput) (Booking, error) {
	return Booking{
		ID:             1001,
		RestaurantID:   in.RestaurantID,
		RestaurantName: "Bistro Nomad",
		StartsAt:       in.StartsAt,
		Guests:         in.Guests,
		Status:         StatusPending,
		SpecialRequest: in.SpecialRequest,
		CreatedAt:      time.Now().UTC(),
	}, nil
}

func (r *Repo) ListForUser(ctx context.Context, userID int64) ([]Booking, error) {
	tomorrow := time.Now().UTC().Truncate(24 * time.Hour).Add(24*time.Hour + 19*time.Hour)
	return []Booking{{
		ID:             1001,
		RestaurantID:   1,
		RestaurantName: "Bistro Nomad",
		StartsAt:       tomorrow,
		Guests:         2,
		Status:         StatusConfirmed,
		CreatedAt:      time.Now().UTC().Add(-time.Hour),
	}}, nil
}

func (r *Repo) Cancel(ctx context.Context, userID, bookingID int64) (Booking, error) {
	if bookingID != 1001 {
		return Booking{}, ErrNotFound
	}
	return Booking{ID: bookingID, RestaurantID: 1, RestaurantName: "Bistro Nomad", Status: StatusCancelled}, nil
}
