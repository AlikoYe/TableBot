package main

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

type ownerBookingJSON struct {
	bookingJSON
	Guest *struct {
		Name     string  `json:"name"`
		Username *string `json:"username"`
	} `json:"guest"`
}

func (e *env) ownerList(user, venueID int64, date time.Time) response {
	e.t.Helper()
	return e.call(http.MethodGet, fmt.Sprintf("/api/owner/bookings?venue_id=%d&date=%s", venueID, date.Format(time.DateOnly)), user, nil)
}

func (e *env) decide(user, bookingID int64, action string) response {
	e.t.Helper()
	return e.call(http.MethodPost, fmt.Sprintf("/api/owner/bookings/%d/%s", bookingID, action), user, nil)
}

func (e *env) newBooking(user, tableID int64, start time.Time) int64 {
	e.t.Helper()
	var b bookingJSON
	e.mustCall(http.MethodPost, "/api/bookings", user, map[string]any{
		"table_id": tableID, "starts_at": start, "guests": 2,
	}, http.StatusCreated, &b)
	return b.ID
}

func TestOwnerAccessIsPerVenue(t *testing.T) {
	e := newEnv(t)
	idB := e.newBooking(2, e.tableB4, e.tomorrow(12, 0))

	// User 1 owns venue A only; venue B belongs to user 99.
	e.wantError(e.ownerList(1, e.venueB, e.tomorrow(0, 0)), 403, "forbidden")
	e.wantError(e.decide(1, idB, "confirm"), 403, "forbidden")
	e.wantError(e.decide(1, idB, "reject"), 403, "forbidden")
	e.wantError(e.decide(2, idB, "confirm"), 403, "forbidden") // the guest is not the owner
	e.wantError(e.ownerList(1, 999999, e.tomorrow(0, 0)), 404, "not_found")
	e.wantError(e.decide(1, 999999, "confirm"), 404, "not_found")
	e.wantError(e.call(http.MethodGet, "/api/owner/bookings?date=2030-01-01", 1, nil), 400, "bad_request")
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/owner/bookings?venue_id=%d", e.venueA), 1, nil), 400, "bad_request")
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/owner/bookings?venue_id=%d&date=2030-01-01", e.venueA), 0, nil), 401, "unauthorized")

	var list []ownerBookingJSON
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/owner/bookings?venue_id=%d&date=%s", e.venueB, e.tomorrow(0, 0).Format(time.DateOnly)), 99, nil, 200, &list)
	if len(list) != 1 || list[0].ID != idB || list[0].Status != "pending" {
		t.Fatalf("owner 99 list = %+v", list)
	}
}

func TestOwnerConfirmAndReject(t *testing.T) {
	e := newEnv(t)
	id := e.newBooking(2, e.tableA4, e.tomorrow(19, 0))

	var b ownerBookingJSON
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/owner/bookings/%d/confirm", id), 1, nil, 200, &b)
	if b.Status != "confirmed" || b.ExpiresAt != nil || b.Guest == nil || b.Guest.Name != "Dev 2" || b.Guest.Username == nil || *b.Guest.Username != "dev_2" {
		t.Fatalf("confirmed booking = %+v guest=%+v", b, b.Guest)
	}
	var decidedAt *time.Time
	if err := e.db.QueryRow(t.Context(), `SELECT decided_at FROM bookings WHERE id = $1`, id).Scan(&decidedAt); err != nil || decidedAt == nil {
		t.Fatalf("decided_at = %v, %v", decidedAt, err)
	}
	if len(e.notifier.decided) != 1 || e.notifier.decided[0] != fmt.Sprintf("%d:confirmed", id) {
		t.Errorf("notifier.decided = %v", e.notifier.decided)
	}

	var mine []bookingJSON
	e.mustCall(http.MethodGet, "/api/bookings/my", 2, nil, 200, &mine)
	if len(mine) != 1 || mine[0].Status != "confirmed" {
		t.Fatalf("guest sees %+v, want confirmed", mine)
	}

	e.wantError(e.decide(1, id, "confirm"), 409, "not_pending")
	e.wantError(e.decide(1, id, "reject"), 409, "not_pending")

	// Overdue hold: still 'pending' in the row, but no longer decidable.
	past := time.Now().Add(-time.Minute)
	overdue := e.insertBooking(e.venueA, e.tableA6, 3, e.tomorrow(12, 0), "pending", &past)
	e.wantError(e.decide(1, overdue, "confirm"), 409, "not_pending")

	other := e.newBooking(4, e.tableA6, e.tomorrow(19, 0))
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/owner/bookings/%d/reject", other), 1, nil, 200, &b)
	if b.Status != "rejected" || b.Guest == nil {
		t.Fatalf("rejected booking = %+v", b)
	}
	// A rejected booking frees the table.
	e.newBooking(5, e.tableA6, e.tomorrow(19, 0))
}

func TestOwnerListLocalDateAndEffectiveStatus(t *testing.T) {
	e := newEnv(t)
	day := e.tomorrow(0, 0)
	past := time.Now().Add(-time.Minute)
	// 00:30 local tomorrow is still "today" in UTC (19:30Z) — it must be listed under tomorrow.
	early := e.insertBooking(e.venueA, e.tableA2, 10, day.Add(30*time.Minute), "confirmed", nil)
	overdue := e.insertBooking(e.venueA, e.tableA4, 11, e.tomorrow(12, 0), "pending", &past)
	cancelled := e.insertBooking(e.venueA, e.tableA6, 12, e.tomorrow(19, 0), "cancelled", nil)
	e.insertBooking(e.venueA, e.tableA6, 13, day.AddDate(0, 0, 1).Add(30*time.Minute), "confirmed", nil) // next day
	e.insertBooking(e.venueA, e.tableA6, 14, day.Add(-90*time.Minute), "confirmed", nil)                 // previous day 22:30
	e.insertBooking(e.venueB, e.tableB4, 15, e.tomorrow(12, 0), "confirmed", nil)                        // other venue

	var list []ownerBookingJSON
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/owner/bookings?venue_id=%d&date=%s", e.venueA, day.Format(time.DateOnly)), 1, nil, 200, &list)
	want := []struct {
		id     int64
		status string
	}{{early, "confirmed"}, {overdue, "expired"}, {cancelled, "cancelled"}}
	if len(list) != len(want) {
		t.Fatalf("got %d bookings, want %d: %+v", len(list), len(want), list)
	}
	for i, w := range want {
		if list[i].ID != w.id || list[i].Status != w.status || list[i].Guest == nil {
			t.Errorf("item %d = id %d %s guest=%v, want id %d %s", i, list[i].ID, list[i].Status, list[i].Guest, w.id, w.status)
		}
	}
	if _, off := list[0].StartsAt.Zone(); off != 5*3600 {
		t.Errorf("starts_at offset %d, want venue-local +05:00", off)
	}
}

func TestConfirmAndRejectRace(t *testing.T) {
	e := newEnv(t)
	for round := range 6 { // 2-hour slots 10:00..20:00
		id := e.newBooking(int64(200+round), e.tableA4, e.tomorrow(10+2*round, 0))
		results := make([]response, 2)
		var wg sync.WaitGroup
		for i, action := range []string{"confirm", "reject"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i] = e.decide(1, id, action)
			}()
		}
		wg.Wait()
		ok, conflict := 0, 0
		for _, r := range results {
			switch {
			case r.code == 200:
				ok++
			case r.code == 409 && r.errorCode() == "not_pending":
				conflict++
			default:
				t.Errorf("round %d: unexpected %d %s", round, r.code, r.body)
			}
		}
		if ok != 1 || conflict != 1 {
			t.Fatalf("round %d: %d ok, %d not_pending; want exactly one of each", round, ok, conflict)
		}
	}
}
