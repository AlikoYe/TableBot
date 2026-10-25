package venue_test

import (
	"context"
	"testing"
	"time"

	"github.com/AlikoYe/TableBot/backend/internal/platform/dbtest"
	"github.com/AlikoYe/TableBot/backend/internal/venue"
)

func TestVenueClosingAt2400ScansFromPostgres(t *testing.T) {
	db := dbtest.New(t)
	ctx := context.Background()
	var id int64
	err := db.QueryRow(ctx, `
		INSERT INTO venues (name, category, opens_at, closes_at, booking_minutes, slot_step_minutes)
		VALUES ('Late', 'bar', '18:00', '24:00', 120, 30) RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}

	v, err := venue.NewRepo(db).Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if v.OpensAt != "18:00" || v.ClosesAt != "24:00" || v.ClosesMinute != 1440 {
		t.Fatalf("scanned opens=%q closes=%q closesMinute=%d", v.OpensAt, v.ClosesAt, v.ClosesMinute)
	}
	sched, err := v.Schedule()
	if err != nil {
		t.Fatal(err)
	}
	starts := sched.StartTimes(2030, 6, 1, time.Time{})
	if last := starts[len(starts)-1].Format("15:04"); last != "22:00" {
		t.Fatalf("last slot = %s, want 22:00", last)
	}
}
