package owner

import (
	"context"
	"errors"

	"github.com/AlikoYe/TableBot/backend/internal/venue"
)

// UpdateVenue changes the settings of a venue the user owns. Existing bookings are not touched.
func (s *Service) UpdateVenue(ctx context.Context, userID, venueID int64, p venue.VenuePatch) (venue.Venue, error) {
	if err := s.CheckVenue(ctx, userID, venueID); err != nil {
		return venue.Venue{}, err
	}
	v, err := s.venues.Get(ctx, venueID)
	if err != nil {
		return venue.Venue{}, notFound(err)
	}
	v, err = venue.ApplyVenuePatch(v, p)
	if err != nil {
		return venue.Venue{}, err
	}
	if err := s.venues.UpdateVenue(ctx, v); err != nil {
		return venue.Venue{}, notFound(err)
	}
	return s.venues.Get(ctx, venueID)
}

// Tables lists every table of a venue the user owns, inactive ones included.
func (s *Service) Tables(ctx context.Context, userID, venueID int64) ([]venue.EditableTable, error) {
	if err := s.CheckVenue(ctx, userID, venueID); err != nil {
		return nil, err
	}
	return s.venues.AllTables(ctx, venueID)
}

// CreateTable adds a table to a venue the user owns.
func (s *Service) CreateTable(ctx context.Context, userID, venueID int64, p venue.TablePatch) (venue.EditableTable, error) {
	if err := s.CheckVenue(ctx, userID, venueID); err != nil {
		return venue.EditableTable{}, err
	}
	t, err := venue.NewTable(p)
	if err != nil {
		return venue.EditableTable{}, err
	}
	return s.venues.CreateTable(ctx, venueID, t)
}

// UpdateTable edits or (de)activates a table. Deactivated tables keep their existing
// bookings but cannot be booked again.
func (s *Service) UpdateTable(ctx context.Context, userID, tableID int64, p venue.TablePatch) (venue.EditableTable, error) {
	venueID, t, err := s.venues.TableForEdit(ctx, tableID)
	if err != nil {
		return venue.EditableTable{}, notFound(err)
	}
	if err := s.CheckVenue(ctx, userID, venueID); err != nil {
		return venue.EditableTable{}, err
	}
	t, err = venue.ApplyTablePatch(t, p)
	if err != nil {
		return venue.EditableTable{}, err
	}
	t, err = s.venues.UpdateTable(ctx, t)
	return t, notFound(err)
}

func notFound(err error) error {
	if errors.Is(err, venue.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
