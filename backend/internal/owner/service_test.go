package owner_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/AlikoYe/TableBot/backend/internal/owner"
	"github.com/AlikoYe/TableBot/backend/internal/platform/dbtest"
)

func TestOwnershipChecks(t *testing.T) {
	db := dbtest.New(t)
	ctx := context.Background()
	svc := owner.NewService(owner.NewRepo(db))

	// User 1 owns venue A; user 2 owns nothing; venue C has no owner.
	var venueA, venueC, tableA, bookingA int64
	mustScan(t, db.QueryRow(ctx, `INSERT INTO users (telegram_id) VALUES (1), (2) RETURNING 0`).Scan(new(int)))
	mustScan(t, db.QueryRow(ctx, `INSERT INTO venues (owner_id, name, category, opens_at, closes_at)
		VALUES (1, 'A', 'cafe', '08:00', '22:00') RETURNING id`).Scan(&venueA))
	mustScan(t, db.QueryRow(ctx, `INSERT INTO venues (name, category, opens_at, closes_at)
		VALUES ('C', 'bar', '18:00', '24:00') RETURNING id`).Scan(&venueC))
	mustScan(t, db.QueryRow(ctx, `INSERT INTO venue_tables (venue_id, label, seats, x, y)
		VALUES ($1, 'T1', 2, 10, 10) RETURNING id`, venueA).Scan(&tableA))
	mustScan(t, db.QueryRow(ctx, `INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status)
		VALUES ($1, $2, 2, 2, '[2030-06-01 10:00+05, 2030-06-01 11:30+05)', 'confirmed') RETURNING id`,
		venueA, tableA).Scan(&bookingA))

	ids, err := svc.OwnedVenueIDs(ctx, 1)
	if err != nil || !slices.Equal(ids, []int64{venueA}) {
		t.Fatalf("OwnedVenueIDs(1) = %v, %v", ids, err)
	}
	if ids, _ := svc.OwnedVenueIDs(ctx, 2); len(ids) != 0 {
		t.Fatalf("OwnedVenueIDs(2) = %v, want none", ids)
	}

	venueTests := []struct {
		user, venue int64
		want        error
	}{
		{1, venueA, nil},
		{2, venueA, owner.ErrForbidden},
		{1, venueC, owner.ErrForbidden},
		{1, 999999, owner.ErrNotFound},
	}
	for _, tt := range venueTests {
		if err := svc.CheckVenue(ctx, tt.user, tt.venue); !errors.Is(err, tt.want) {
			t.Errorf("CheckVenue(user %d, venue %d) = %v, want %v", tt.user, tt.venue, err, tt.want)
		}
	}

	if v, err := svc.CheckBooking(ctx, 1, bookingA); err != nil || v != venueA {
		t.Errorf("CheckBooking(owner) = %d, %v", v, err)
	}
	// The guest who made the booking is not the venue owner.
	if _, err := svc.CheckBooking(ctx, 2, bookingA); !errors.Is(err, owner.ErrForbidden) {
		t.Errorf("CheckBooking(guest) = %v, want ErrForbidden", err)
	}
	if _, err := svc.CheckBooking(ctx, 1, 999999); !errors.Is(err, owner.ErrNotFound) {
		t.Errorf("CheckBooking(missing) = %v, want ErrNotFound", err)
	}
}

func mustScan(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
