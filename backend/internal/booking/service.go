// Package booking handles customer reservations.
package booking

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound  = errors.New("booking not found")
	ErrSlotFull  = errors.New("the selected time slot has no room for this party")
	ErrNotOwner  = errors.New("booking belongs to another user")
	ErrNotActive = errors.New("booking can no longer be changed")
)

// Status values mirror the bookings.status CHECK constraint.
const (
	StatusPending   = "pending"
	StatusConfirmed = "confirmed"
	StatusCancelled = "cancelled"
	StatusCompleted = "completed"
	StatusNoShow    = "no_show"
)

type Booking struct {
	ID             int64     `json:"id"`
	RestaurantID   int64     `json:"restaurant_id"`
	RestaurantName string    `json:"restaurant_name"`
	StartsAt       time.Time `json:"starts_at"`
	Guests         int       `json:"guests"`
	Status         string    `json:"status"`
	SpecialRequest string    `json:"special_request"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateInput struct {
	UserID         int64
	RestaurantID   int64
	StartsAt       time.Time
	Guests         int
	SpecialRequest string
}

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Create reserves a slot for the user.
// TODO: upsert user, lock the time_slots row, check capacity, insert the
// booking and bump booked_guests in one transaction; map constraint
// violations to ErrSlotFull.
func (s *Service) Create(ctx context.Context, in CreateInput) (Booking, error) {
	return s.repo.Create(ctx, in)
}

func (s *Service) ListForUser(ctx context.Context, userID int64) ([]Booking, error) {
	return s.repo.ListForUser(ctx, userID)
}

// Cancel cancels the user's own booking.
// TODO: verify ownership and status, release slot capacity.
func (s *Service) Cancel(ctx context.Context, userID, bookingID int64) (Booking, error) {
	return s.repo.Cancel(ctx, userID, bookingID)
}
