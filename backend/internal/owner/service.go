// Package owner serves venue owners: their venue's bookings, confirm and reject.
// Ownership is per venue (venues.owner_id), not a global role.
package owner

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/AlikoYe/TableBot/backend/internal/booking"
	"github.com/AlikoYe/TableBot/backend/internal/bookingstatus"
	"github.com/AlikoYe/TableBot/backend/internal/venue"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("you are not the owner of this venue")
	ErrNotPending = errors.New("booking is no longer pending")
)

// Notifier tells the guest about the owner's decision. Called after the update commits.
type Notifier interface {
	BookingDecided(ctx context.Context, b booking.Booking) error
}

type Service struct {
	repo     *Repo
	bookings *booking.Repo
	notifier Notifier
	log      *slog.Logger
}

func NewService(repo *Repo, bookings *booking.Repo, notifier Notifier, log *slog.Logger) *Service {
	return &Service{repo: repo, bookings: bookings, notifier: notifier, log: log}
}

// OwnedVenueIDs implements auth.VenueOwnership.
func (s *Service) OwnedVenueIDs(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.OwnedVenueIDs(ctx, userID)
}

// CheckVenue returns nil if userID owns venueID, ErrForbidden if not, ErrNotFound if there is no such venue.
func (s *Service) CheckVenue(ctx context.Context, userID, venueID int64) error {
	ownerID, _, err := s.repo.VenueOwner(ctx, venueID)
	if err != nil {
		return err
	}
	if ownerID != userID {
		return ErrForbidden
	}
	return nil
}

// CheckBooking is CheckVenue for the venue of a booking; it returns that venue's id.
func (s *Service) CheckBooking(ctx context.Context, userID, bookingID int64) (int64, error) {
	venueID, ownerID, err := s.repo.BookingVenueOwner(ctx, bookingID)
	if err != nil {
		return 0, err
	}
	if ownerID != userID {
		return 0, ErrForbidden
	}
	return venueID, nil
}

// List returns all bookings starting on the venue-local date, with guests.
func (s *Service) List(ctx context.Context, userID, venueID int64, date time.Time) ([]booking.Booking, error) {
	ownerID, tz, err := s.repo.VenueOwner(ctx, venueID)
	if err != nil {
		return nil, err
	}
	if ownerID != userID {
		return nil, ErrForbidden
	}
	loc, err := venue.Location(tz)
	if err != nil {
		return nil, err
	}
	y, m, d := date.Date()
	from := time.Date(y, m, d, 0, 0, 0, 0, loc)
	items, err := s.bookings.ListForVenue(ctx, venueID, from, from.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i] = items[i].WithGuest()
	}
	return items, nil
}

// Decide confirms or rejects a pending booking as actorID. Used by HTTP and the bot.
func (s *Service) Decide(ctx context.Context, actorID, bookingID int64, confirm bool) (booking.Booking, error) {
	if _, err := s.CheckBooking(ctx, actorID, bookingID); err != nil {
		return booking.Booking{}, err
	}
	status := bookingstatus.Rejected
	if confirm {
		status = bookingstatus.Confirmed
	}
	ok, err := s.repo.Decide(ctx, bookingID, status)
	if err != nil {
		return booking.Booking{}, err
	}
	if !ok {
		return booking.Booking{}, ErrNotPending
	}
	b, err := s.bookings.Get(ctx, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	if err := s.notifier.BookingDecided(ctx, b); err != nil {
		s.log.WarnContext(ctx, "notify booking decided", slog.Int64("booking_id", b.ID), slog.Any("err", err))
	}
	return b.WithGuest(), nil
}
