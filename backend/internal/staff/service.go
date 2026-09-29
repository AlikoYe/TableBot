// Package staff serves restaurant employees: today's bookings, confirmations,
// and role lookup.
package staff

import (
	"context"
	"errors"
	"time"
)

// Roles mirror the users.role CHECK constraint.
const (
	RoleCustomer = "customer"
	RoleStaff    = "staff"
	RoleAdmin    = "admin"
)

var ErrNotFound = errors.New("booking not found")

// Booking is the staff-facing view of a reservation.
type Booking struct {
	ID             int64     `json:"id"`
	RestaurantID   int64     `json:"restaurant_id"`
	StartsAt       time.Time `json:"starts_at"`
	Guests         int       `json:"guests"`
	Status         string    `json:"status"`
	SpecialRequest string    `json:"special_request"`
	CustomerName   string    `json:"customer_name"`
}

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// RoleFor implements auth.RoleResolver.
func (s *Service) RoleFor(ctx context.Context, telegramID int64) (string, error) {
	return s.repo.RoleFor(ctx, telegramID)
}

func IsStaffRole(role string) bool {
	return role == RoleStaff || role == RoleAdmin
}

// BookingsForDate lists bookings at the restaurants the staff member works at.
// TODO: scope by restaurant_staff, interpret date in restaurant timezone.
func (s *Service) BookingsForDate(ctx context.Context, staffID int64, date time.Time) ([]Booking, error) {
	return s.repo.BookingsForDate(ctx, staffID, date)
}

// Confirm marks a pending booking as confirmed.
// TODO: check the booking belongs to one of the staff member's restaurants,
// check status transition, notify the customer via bot.Notifier.
func (s *Service) Confirm(ctx context.Context, staffID, bookingID int64) (Booking, error) {
	return s.repo.Confirm(ctx, staffID, bookingID)
}
