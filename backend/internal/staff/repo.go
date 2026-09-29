package staff

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo is the PostgreSQL data access layer for staff features.
type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

// RoleFor returns the user's global role; unknown users are customers.
func (r *Repo) RoleFor(ctx context.Context, telegramID int64) (string, error) {
	var role string
	err := r.db.QueryRow(ctx, `SELECT role FROM users WHERE telegram_id = $1`, telegramID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return RoleCustomer, nil
	}
	if err != nil {
		return "", fmt.Errorf("select user role: %w", err)
	}
	return role, nil
}

// TODO: replace the mock data below with real queries.

func (r *Repo) BookingsForDate(ctx context.Context, staffID int64, date time.Time) ([]Booking, error) {
	at := func(h, m int) time.Time {
		return time.Date(date.Year(), date.Month(), date.Day(), h, m, 0, 0, time.UTC)
	}
	return []Booking{
		{ID: 1001, RestaurantID: 1, StartsAt: at(19, 0), Guests: 2, Status: "pending", CustomerName: "Ada L."},
		{ID: 1002, RestaurantID: 1, StartsAt: at(20, 30), Guests: 4, Status: "confirmed", SpecialRequest: "Window table", CustomerName: "Alan T."},
	}, nil
}

func (r *Repo) Confirm(ctx context.Context, staffID, bookingID int64) (Booking, error) {
	if bookingID != 1001 && bookingID != 1002 {
		return Booking{}, ErrNotFound
	}
	return Booking{ID: bookingID, RestaurantID: 1, Status: "confirmed"}, nil
}
