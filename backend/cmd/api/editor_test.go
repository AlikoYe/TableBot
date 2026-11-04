package main

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

type editableTableJSON struct {
	ID       int64   `json:"id"`
	Label    string  `json:"label"`
	Seats    int     `json:"seats"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Shape    string  `json:"shape"`
	IsActive bool    `json:"is_active"`
}

func (e *env) publicTableIDs(venueID int64, start time.Time) map[int64]bool {
	e.t.Helper()
	var m struct {
		Tables []struct {
			ID int64 `json:"id"`
		} `json:"tables"`
	}
	path := fmt.Sprintf("/api/venues/%d/tables?starts_at=%s&guests=1", venueID, url.QueryEscape(start.Format(time.RFC3339)))
	e.mustCall(http.MethodGet, path, 2, nil, 200, &m)
	ids := map[int64]bool{}
	for _, t := range m.Tables {
		ids[t.ID] = true
	}
	return ids
}

func TestOwnerTablesAccess(t *testing.T) {
	e := newEnv(t)
	var tables []editableTableJSON
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/owner/venues/%d/tables", e.venueA), 1, nil, 200, &tables)
	if len(tables) != 4 {
		t.Fatalf("got %d tables, want 4 incl. the inactive one", len(tables))
	}
	if off := tables[3]; off.ID != e.tableOff || off.IsActive {
		t.Fatalf("inactive table = %+v", off)
	}

	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/owner/venues/%d/tables", e.venueA), 99, nil), 403, "forbidden")
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/owner/venues/%d/tables", e.venueA), 2, nil), 403, "forbidden")
	e.wantError(e.call(http.MethodGet, "/api/owner/venues/999999/tables", 1, nil), 404, "not_found")
	e.wantError(e.call(http.MethodPatch, fmt.Sprintf("/api/owner/tables/%d", e.tableB4), 1, map[string]any{"seats": 2}), 403, "forbidden")
	e.wantError(e.call(http.MethodPatch, "/api/owner/tables/999999", 1, map[string]any{"seats": 2}), 404, "not_found")
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/owner/venues/%d/tables", e.venueB), 1,
		map[string]any{"label": "X1", "seats": 2, "x": 10, "y": 10}), 403, "forbidden")
}

func TestOwnerCreateTable(t *testing.T) {
	e := newEnv(t)
	path := fmt.Sprintf("/api/owner/venues/%d/tables", e.venueA)

	var created editableTableJSON
	e.mustCall(http.MethodPost, path, 1, map[string]any{"label": " T9 ", "seats": 8, "x": 33.333, "y": 66.666}, 201, &created)
	want := editableTableJSON{ID: created.ID, Label: "T9", Seats: 8, X: 33.33, Y: 66.67, Shape: "round", IsActive: true}
	if created != want {
		t.Fatalf("created %+v, want %+v", created, want)
	}
	if !e.publicTableIDs(e.venueA, e.tomorrow(19, 0))[created.ID] {
		t.Fatal("new table is not visible to guests")
	}
	if r := e.book(2, created.ID, e.tomorrow(19, 0), 8); r.code != 201 {
		t.Fatalf("booking the new table: %d %s", r.code, r.body)
	}

	e.wantError(e.call(http.MethodPost, path, 1, map[string]any{"label": "T1", "seats": 2, "x": 1, "y": 1}), 409, "label_taken")
	for name, body := range map[string]map[string]any{
		"missing y":  {"label": "N1", "seats": 2, "x": 1},
		"seats 0":    {"label": "N1", "seats": 0, "x": 1, "y": 1},
		"seats 21":   {"label": "N1", "seats": 21, "x": 1, "y": 1},
		"x over 100": {"label": "N1", "seats": 2, "x": 101, "y": 1},
		"bad shape":  {"label": "N1", "seats": 2, "x": 1, "y": 1, "shape": "oval"},
		"long label": {"label": "ABCDEFGHIJK", "seats": 2, "x": 1, "y": 1},
		"blank":      {"label": "   ", "seats": 2, "x": 1, "y": 1},
	} {
		t.Run(name, func(t *testing.T) {
			e.wantError(e.call(http.MethodPost, path, 1, body), 400, "bad_request")
		})
	}
}

func TestOwnerUpdateTable(t *testing.T) {
	e := newEnv(t)
	path := fmt.Sprintf("/api/owner/tables/%d", e.tableA4)
	start := e.tomorrow(19, 0)

	var moved editableTableJSON
	e.mustCall(http.MethodPatch, path, 1, map[string]any{"x": 40, "y": 45.5, "seats": 6, "shape": "rect"}, 200, &moved)
	if moved.X != 40 || moved.Y != 45.5 || moved.Seats != 6 || moved.Shape != "rect" || moved.Label != "T2" || !moved.IsActive {
		t.Fatalf("moved = %+v", moved)
	}
	e.wantError(e.call(http.MethodPatch, path, 1, map[string]any{"label": "T1"}), 409, "label_taken")
	e.wantError(e.call(http.MethodPatch, path, 1, map[string]any{"seats": 0}), 400, "bad_request")

	// An existing booking survives deactivation; new ones are refused.
	kept := e.insertBooking(e.venueA, e.tableA4, 20, start, "confirmed", nil)
	e.mustCall(http.MethodPatch, path, 1, map[string]any{"is_active": false}, 200, &moved)
	if moved.IsActive {
		t.Fatal("table still active")
	}
	if e.publicTableIDs(e.venueA, start)[e.tableA4] {
		t.Fatal("inactive table is still shown to guests")
	}
	e.wantError(e.book(2, e.tableA4, e.tomorrow(12, 0), 2), 404, "not_found")
	var mine []bookingJSON
	e.mustCall(http.MethodGet, "/api/bookings/my", 20, nil, 200, &mine)
	if len(mine) != 1 || mine[0].ID != kept || mine[0].Status != "confirmed" {
		t.Fatalf("existing booking after deactivation = %+v", mine)
	}

	e.mustCall(http.MethodPatch, path, 1, map[string]any{"is_active": true}, 200, &moved)
	if r := e.book(2, e.tableA4, e.tomorrow(12, 0), 2); r.code != 201 {
		t.Fatalf("booking the reactivated table: %d %s", r.code, r.body)
	}
}

func TestOwnerUpdateVenue(t *testing.T) {
	e := newEnv(t)
	path := fmt.Sprintf("/api/owner/venues/%d", e.venueA)
	kept := e.insertBooking(e.venueA, e.tableA4, 20, e.tomorrow(10, 0), "confirmed", nil)

	var v struct {
		Name            string `json:"name"`
		Description     string `json:"description"`
		OpensAt         string `json:"opens_at"`
		ClosesAt        string `json:"closes_at"`
		BookingMinutes  int    `json:"booking_minutes"`
		SlotStepMinutes int    `json:"slot_step_minutes"`
	}
	e.mustCall(http.MethodPatch, path, 1, map[string]any{
		"name": " New name ", "description": "Updated", "opens_at": "12:00", "closes_at": "22:00",
		"booking_minutes": 90, "slot_step_minutes": 60,
	}, 200, &v)
	if v.Name != "New name" || v.Description != "Updated" || v.OpensAt != "12:00" || v.ClosesAt != "22:00" ||
		v.BookingMinutes != 90 || v.SlotStepMinutes != 60 {
		t.Fatalf("venue after update = %+v", v)
	}

	var times []struct {
		StartsAt time.Time `json:"starts_at"`
	}
	date := e.tomorrow(0, 0).Format(time.DateOnly)
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/venues/%d/times?date=%s", e.venueA, date), 2, nil, 200, &times)
	// 12:00, 13:00 ... 20:00 (a 90-minute booking must end by 22:00).
	if len(times) != 9 || times[0].StartsAt.In(e.loc).Format("15:04") != "12:00" || times[8].StartsAt.In(e.loc).Format("15:04") != "20:00" {
		t.Fatalf("times after update: %d, first %v", len(times), times)
	}

	var mine []bookingJSON
	e.mustCall(http.MethodGet, "/api/bookings/my", 20, nil, 200, &mine)
	if len(mine) != 1 || mine[0].ID != kept || mine[0].Status != "confirmed" {
		t.Fatalf("existing booking after hours change = %+v", mine)
	}

	e.mustCall(http.MethodPatch, path, 1, map[string]any{"closes_at": "24:00"}, 200, &v)
	if v.ClosesAt != "24:00" || v.OpensAt != "12:00" {
		t.Fatalf("partial update = %+v", v)
	}

	for name, body := range map[string]map[string]any{
		"closes before opens":     {"opens_at": "20:00", "closes_at": "19:00"},
		"opens 24:00":             {"opens_at": "24:00"},
		"bad clock":               {"opens_at": "9:00"},
		"step 20":                 {"slot_step_minutes": 20},
		"booking not 15 min":      {"booking_minutes": 50},
		"booking longer than day": {"opens_at": "12:00", "closes_at": "13:00", "booking_minutes": 90},
		"empty name":              {"name": "  "},
	} {
		t.Run(name, func(t *testing.T) {
			e.wantError(e.call(http.MethodPatch, path, 1, body), 400, "bad_request")
		})
	}

	e.wantError(e.call(http.MethodPatch, path, 99, map[string]any{"name": "x"}), 403, "forbidden")
	e.wantError(e.call(http.MethodPatch, "/api/owner/venues/999999", 1, map[string]any{"name": "x"}), 404, "not_found")
	e.wantError(e.call(http.MethodPatch, path, 0, map[string]any{"name": "x"}), 401, "unauthorized")
}
