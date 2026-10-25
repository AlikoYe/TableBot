// Package booking handles customer bookings of a specific table.
package booking

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlikoYe/TableBot/backend/internal/venue"
)

// MyBookingsLimit caps GET /bookings/my.
const MyBookingsLimit = 100

var (
	ErrNotFound       = errors.New("booking not found")
	ErrTableNotFound  = errors.New("table not found")
	ErrTooManyGuests  = errors.New("too many guests for this table")
	ErrTableTaken     = errors.New("this table is already booked for that time")
	ErrUserBusy       = errors.New("you already have a booking at that time")
	ErrNotCancellable = errors.New("booking can no longer be cancelled")
)

type Booking struct {
	ID             int64      `json:"id"`
	Status         string     `json:"status"`
	Guests         int        `json:"guests"`
	StartsAt       time.Time  `json:"starts_at"`
	EndsAt         time.Time  `json:"ends_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	SpecialRequest string     `json:"special_request"`
	CreatedAt      time.Time  `json:"created_at"`
	Venue          VenueRef   `json:"venue"`
	Table          TableRef   `json:"table"`
	Guest          *Guest     `json:"guest,omitempty"`

	UserID    int64 `json:"-"`
	OwnerID   int64 `json:"-"` // venue owner; 0 when the venue has none
	GuestInfo Guest `json:"-"` // always loaded; exposed via WithGuest
}

// WithGuest returns the booking with the guest field filled, for owner responses.
func (b Booking) WithGuest() Booking {
	g := b.GuestInfo
	b.Guest = &g
	return b
}

type VenueRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type TableRef struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
	Seats int    `json:"seats"`
}

// Guest is only filled in owner responses.
type Guest struct {
	Name     string  `json:"name"`
	Username *string `json:"username"`
}

// Notifier is told about booking changes after they are committed. Implemented by bot.Notifier.
type Notifier interface {
	BookingCreated(ctx context.Context, b Booking) error
	BookingCancelled(ctx context.Context, b Booking) error
}

type CreateInput struct {
	UserID         int64
	TableID        int64
	StartsAt       time.Time
	Guests         int
	SpecialRequest string
}

type Service struct {
	repo        *Repo
	venues      *venue.Service
	notifier    Notifier
	holdMinutes int
	log         *slog.Logger
	now         func() time.Time
}

func NewService(repo *Repo, venues *venue.Service, notifier Notifier, holdMinutes int, log *slog.Logger) *Service {
	return &Service{repo: repo, venues: venues, notifier: notifier, holdMinutes: holdMinutes, log: log, now: time.Now}
}

// Create validates the request and inserts a pending booking that holds the table for holdMinutes.
func (s *Service) Create(ctx context.Context, in CreateInput) (Booking, error) {
	v, t, err := s.venues.BookableTable(ctx, in.TableID)
	if errors.Is(err, venue.ErrNotFound) {
		return Booking{}, ErrTableNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	if in.Guests > t.Seats {
		return Booking{}, fmt.Errorf("%w: table %s seats %d", ErrTooManyGuests, t.Label, t.Seats)
	}
	sched, err := v.Schedule()
	if err != nil {
		return Booking{}, err
	}
	if err := sched.Check(in.StartsAt, s.now()); err != nil {
		return Booking{}, err
	}

	id, err := s.repo.Insert(ctx, insertRow{
		VenueID:        v.ID,
		TableID:        t.ID,
		UserID:         in.UserID,
		Guests:         in.Guests,
		StartsAt:       in.StartsAt,
		BookingMinutes: v.BookingMinutes,
		HoldMinutes:    s.holdMinutes,
		SpecialRequest: in.SpecialRequest,
	})
	if err != nil {
		return Booking{}, err
	}
	b, err := s.repo.Get(ctx, id)
	if err != nil {
		return Booking{}, err
	}
	if err := s.notifier.BookingCreated(ctx, b); err != nil {
		s.log.WarnContext(ctx, "notify booking created", slog.Int64("booking_id", b.ID), slog.Any("err", err))
	}
	return b, nil
}

func (s *Service) ListForUser(ctx context.Context, userID int64) ([]Booking, error) {
	return s.repo.ListForUser(ctx, userID, MyBookingsLimit)
}

// Cancel cancels the user's own pending or confirmed booking that has not started.
func (s *Service) Cancel(ctx context.Context, userID, bookingID int64) (Booking, error) {
	if err := s.repo.Cancel(ctx, userID, bookingID); err != nil {
		return Booking{}, err
	}
	b, err := s.repo.Get(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if err := s.notifier.BookingCancelled(ctx, b); err != nil {
		s.log.WarnContext(ctx, "notify booking cancelled", slog.Int64("booking_id", b.ID), slog.Any("err", err))
	}
	return b, nil
}
