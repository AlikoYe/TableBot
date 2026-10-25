// Package owner serves venue owners: their venue's bookings, confirm and reject.
// Ownership is per venue (venues.owner_id), not a global role.
package owner

import (
	"context"
	"errors"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("you are not the owner of this venue")
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// OwnedVenueIDs implements auth.VenueOwnership.
func (s *Service) OwnedVenueIDs(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.OwnedVenueIDs(ctx, userID)
}

// CheckVenue returns nil if userID owns venueID, ErrForbidden if not, ErrNotFound if there is no such venue.
func (s *Service) CheckVenue(ctx context.Context, userID, venueID int64) error {
	ownerID, err := s.repo.VenueOwner(ctx, venueID)
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
