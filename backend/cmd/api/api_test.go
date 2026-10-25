package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"

	"github.com/AlikoYe/TableBot/backend/internal/booking"
	"github.com/AlikoYe/TableBot/backend/internal/platform/config"
	"github.com/AlikoYe/TableBot/backend/internal/platform/dbtest"
)

// contract validates every response in these tests against docs/api.yaml.
var (
	contractOnce sync.Once
	contract     validator.Validator
	contractErr  error
	contractMu   sync.Mutex
)

func loadContract(t *testing.T) validator.Validator {
	t.Helper()
	contractOnce.Do(func() {
		spec, err := os.ReadFile("../../../docs/api.yaml")
		if err != nil {
			contractErr = err
			return
		}
		doc, err := libopenapi.NewDocument(spec)
		if err != nil {
			contractErr = err
			return
		}
		v, errs := validator.NewValidator(doc)
		if len(errs) > 0 {
			contractErr = errors.Join(errs...)
			return
		}
		contract = v
	})
	if contractErr != nil {
		t.Fatalf("load docs/api.yaml: %v", contractErr)
	}
	return contract
}

type fakeNotifier struct {
	mu        sync.Mutex
	created   []int64
	cancelled []int64
	err       error
}

func (f *fakeNotifier) BookingCreated(_ context.Context, b booking.Booking) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, b.ID)
	return f.err
}

func (f *fakeNotifier) BookingCancelled(_ context.Context, b booking.Booking) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelled = append(f.cancelled, b.ID)
	return f.err
}

type env struct {
	t        *testing.T
	db       *pgxpool.Pool
	h        http.Handler
	notifier *fakeNotifier
	loc      *time.Location

	venueA, venueB, venueOff                     int64
	tableA2, tableA4, tableA6, tableOff, tableB4 int64
}

// newEnv migrates a clean DB and seeds venue A (10:00-24:00, 120 min, tables of 2/4/6 seats
// plus an inactive one), venue B (08:00-22:00, 90 min) and an inactive venue.
func newEnv(t *testing.T) *env {
	t.Helper()
	db := dbtest.New(t)
	loc, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, db: db, notifier: &fakeNotifier{}, loc: loc}
	cfg := config.Config{AppEnv: config.EnvDev, HoldMinutes: 15}
	e.h = newRouter(cfg, slog.New(slog.DiscardHandler), db, e.notifier)
	loadContract(t)

	e.exec(`INSERT INTO users (telegram_id, name) VALUES (1, 'Owner')`)
	e.venueA = e.id(`INSERT INTO venues (owner_id, name, category, opens_at, closes_at, booking_minutes, slot_step_minutes, floor_plan_url)
		VALUES (1, 'Venue A', 'restaurant', '10:00', '24:00', 120, 30, '/api/static/plans/nomad-bistro.svg') RETURNING id`)
	e.venueB = e.id(`INSERT INTO venues (owner_id, name, category, opens_at, closes_at, booking_minutes, slot_step_minutes)
		VALUES (1, 'Venue B', 'cafe', '08:00', '22:00', 90, 30) RETURNING id`)
	e.venueOff = e.id(`INSERT INTO venues (name, category, opens_at, closes_at, is_active)
		VALUES ('Closed', 'bar', '10:00', '20:00', false) RETURNING id`)
	e.tableA2 = e.id(`INSERT INTO venue_tables (venue_id, label, seats, x, y) VALUES ($1, 'T1', 2, 10, 10) RETURNING id`, e.venueA)
	e.tableA4 = e.id(`INSERT INTO venue_tables (venue_id, label, seats, x, y) VALUES ($1, 'T2', 4, 20.5, 30) RETURNING id`, e.venueA)
	e.tableA6 = e.id(`INSERT INTO venue_tables (venue_id, label, seats, x, y, shape) VALUES ($1, 'T3', 6, 50, 50, 'rect') RETURNING id`, e.venueA)
	e.tableOff = e.id(`INSERT INTO venue_tables (venue_id, label, seats, x, y, is_active) VALUES ($1, 'T4', 4, 70, 70, false) RETURNING id`, e.venueA)
	e.tableB4 = e.id(`INSERT INTO venue_tables (venue_id, label, seats, x, y) VALUES ($1, 'B1', 4, 50, 50) RETURNING id`, e.venueB)
	return e
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("exec %q: %v", sql, err)
	}
}

func (e *env) id(sql string, args ...any) int64 {
	e.t.Helper()
	var id int64
	if err := e.db.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		e.t.Fatalf("query %q: %v", sql, err)
	}
	return id
}

// tomorrow returns hh:mm tomorrow in Almaty.
func (e *env) tomorrow(h, m int) time.Time {
	d := time.Now().In(e.loc).AddDate(0, 0, 1)
	return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, e.loc)
}

type response struct {
	code int
	body []byte
}

func (r response) errorCode() string {
	var e struct{ Error string }
	_ = json.Unmarshal(r.body, &e)
	return e.Error
}

// call sends a request as dev user `user` (0 = no auth) and checks the response against the contract.
func (e *env) call(method, path string, user int64, body any) response {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != 0 {
		req.Header.Set("Authorization", fmt.Sprintf("dev %d", user))
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)

	res := rec.Result()
	contractMu.Lock()
	ok, errs := contract.ValidateHttpResponse(req, res)
	contractMu.Unlock()
	if !ok {
		var msgs []string
		for _, ve := range errs {
			msgs = append(msgs, ve.Error())
			for _, se := range ve.SchemaValidationErrors {
				msgs = append(msgs, "  "+se.FieldPath+": "+se.Reason)
			}
		}
		e.t.Errorf("%s %s -> %d does not match docs/api.yaml:\n%s\nbody: %s", method, path, rec.Code, strings.Join(msgs, "\n"), rec.Body)
	}
	return response{code: rec.Code, body: rec.Body.Bytes()}
}

func (e *env) mustCall(method, path string, user int64, body any, wantCode int, out any) response {
	e.t.Helper()
	r := e.call(method, path, user, body)
	if r.code != wantCode {
		e.t.Fatalf("%s %s: status %d, want %d; body %s", method, path, r.code, wantCode, r.body)
	}
	if out != nil {
		if err := json.Unmarshal(r.body, out); err != nil {
			e.t.Fatalf("decode %s: %v", r.body, err)
		}
	}
	return r
}

func (e *env) wantError(r response, code int, errCode string) {
	e.t.Helper()
	if r.code != code || r.errorCode() != errCode {
		e.t.Fatalf("got %d %s, want %d %s", r.code, r.body, code, errCode)
	}
}

type bookingJSON struct {
	ID        int64      `json:"id"`
	Status    string     `json:"status"`
	Guests    int        `json:"guests"`
	StartsAt  time.Time  `json:"starts_at"`
	EndsAt    time.Time  `json:"ends_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	Venue     struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"venue"`
	Table struct {
		ID    int64  `json:"id"`
		Label string `json:"label"`
		Seats int    `json:"seats"`
	} `json:"table"`
}

func (e *env) book(user, tableID int64, start time.Time, guests int) response {
	e.t.Helper()
	return e.call(http.MethodPost, "/api/bookings", user, map[string]any{
		"table_id": tableID, "starts_at": start.Format(time.RFC3339), "guests": guests,
	})
}

// insertBooking writes a booking directly; during is [start, start+2h).
func (e *env) insertBooking(venueID, tableID, user int64, start time.Time, status string, expiresAt *time.Time) int64 {
	e.t.Helper()
	e.exec(`INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING`, user)
	return e.id(`INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status, expires_at)
		VALUES ($1, $2, $3, 2, tstzrange($4::timestamptz, $4::timestamptz + interval '2 hours', '[)'), $5, $6) RETURNING id`,
		venueID, tableID, user, start, status, expiresAt)
}

func TestConcurrentBookingsOfOneTable(t *testing.T) {
	e := newEnv(t)
	for round := 0; round < 3; round++ {
		start := e.tomorrow(12+3*round, 0)
		codes := make([]response, 20)
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				user := int64(1000 + round*100 + i)
				codes[i] = e.book(user, e.tableA4, start, 2)
			}(i)
		}
		wg.Wait()

		created, taken := 0, 0
		for _, r := range codes {
			switch {
			case r.code == http.StatusCreated:
				created++
			case r.code == http.StatusConflict && r.errorCode() == "table_taken":
				taken++
			default:
				t.Errorf("round %d: unexpected response %d %s", round, r.code, r.body)
			}
		}
		if created != 1 || taken != 19 {
			t.Fatalf("round %d: %d created, %d table_taken; want 1 and 19", round, created, taken)
		}
	}
}

func TestConcurrentBookingsByOneUser(t *testing.T) {
	e := newEnv(t)
	tables := []int64{e.tableA2, e.tableA4, e.tableA6, e.tableB4}
	codes := make([]response, len(tables))
	var wg sync.WaitGroup
	for i, tbl := range tables {
		wg.Add(1)
		go func(i int, tbl int64) {
			defer wg.Done()
			codes[i] = e.book(60, tbl, e.tomorrow(19, 0), 2)
		}(i, tbl)
	}
	wg.Wait()
	created, busy := 0, 0
	for _, r := range codes {
		switch {
		case r.code == http.StatusCreated:
			created++
		case r.code == http.StatusConflict && r.errorCode() == "user_busy":
			busy++
		default:
			t.Errorf("unexpected response %d %s", r.code, r.body)
		}
	}
	if created != 1 || busy != 3 {
		t.Fatalf("%d created, %d user_busy; want 1 and 3", created, busy)
	}
}

func TestSameUserOverlappingVenuesIsUserBusy(t *testing.T) {
	e := newEnv(t)
	e.mustCall(http.MethodPost, "/api/bookings", 5, map[string]any{
		"table_id": e.tableA4, "starts_at": e.tomorrow(19, 0), "guests": 2,
	}, http.StatusCreated, nil)
	e.wantError(e.book(5, e.tableB4, e.tomorrow(20, 0), 2), http.StatusConflict, "user_busy")
	// Ends exactly when the other starts: fine.
	e.mustCall(http.MethodPost, "/api/bookings", 5, map[string]any{
		"table_id": e.tableB4, "starts_at": e.tomorrow(17, 30), "guests": 2,
	}, http.StatusCreated, nil)
}

func TestExpiredHoldDoesNotBlockAndIsFlipped(t *testing.T) {
	e := newEnv(t)
	past := time.Now().Add(-time.Minute)
	old := e.insertBooking(e.venueA, e.tableA4, 7, e.tomorrow(19, 0), "pending", &past)

	var b bookingJSON
	e.mustCall(http.MethodPost, "/api/bookings", 8, map[string]any{
		"table_id": e.tableA4, "starts_at": e.tomorrow(19, 0), "guests": 4,
	}, http.StatusCreated, &b)
	if b.Status != "pending" || b.ExpiresAt == nil {
		t.Fatalf("new booking: %+v", b)
	}
	var status string
	if err := e.db.QueryRow(context.Background(), `SELECT status FROM bookings WHERE id = $1`, old).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "expired" {
		t.Fatalf("old hold status = %q, want expired", status)
	}
}

func TestCreateBookingResponse(t *testing.T) {
	e := newEnv(t)
	start := e.tomorrow(19, 0)
	before := time.Now()
	var b bookingJSON
	e.mustCall(http.MethodPost, "/api/bookings", 9, map[string]any{
		"table_id": e.tableA4, "starts_at": start.UTC().Format(time.RFC3339), "guests": 3,
		"special_request": "window please", "unknown_field": true,
	}, http.StatusCreated, &b)

	if b.Status != "pending" || b.Guests != 3 || !b.StartsAt.Equal(start) || !b.EndsAt.Equal(start.Add(2*time.Hour)) {
		t.Errorf("booking = %+v", b)
	}
	if _, off := b.StartsAt.Zone(); off != 5*3600 {
		t.Errorf("starts_at offset = %d, want +05:00 (venue local)", off)
	}
	if b.ExpiresAt == nil || b.ExpiresAt.Before(before.Add(14*time.Minute)) || b.ExpiresAt.After(time.Now().Add(16*time.Minute)) {
		t.Errorf("expires_at = %v, want about now + 15m", b.ExpiresAt)
	}
	if b.Venue.ID != e.venueA || b.Venue.Name != "Venue A" || b.Table.ID != e.tableA4 || b.Table.Label != "T2" || b.Table.Seats != 4 {
		t.Errorf("venue/table = %+v %+v", b.Venue, b.Table)
	}
	if len(e.notifier.created) != 1 || e.notifier.created[0] != b.ID {
		t.Errorf("notifier.created = %v, want [%d]", e.notifier.created, b.ID)
	}
}

func TestCreateBookingValidation(t *testing.T) {
	e := newEnv(t)
	ok := map[string]any{"table_id": e.tableA4, "starts_at": e.tomorrow(19, 0), "guests": 2}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for kk, vv := range ok {
			m[kk] = vv
		}
		m[k] = v
		return m
	}
	tests := []struct {
		name    string
		body    any
		code    int
		errCode string
	}{
		{"missing table", with("table_id", 0), 400, "bad_request"},
		{"zero guests", with("guests", 0), 400, "bad_request"},
		{"21 guests", with("guests", 21), 400, "bad_request"},
		{"bad time", with("starts_at", "tomorrow"), 400, "bad_request"},
		{"special request too long", with("special_request", strings.Repeat("я", 301)), 400, "bad_request"},
		{"unknown table", with("table_id", 999999), 404, "not_found"},
		{"inactive table", with("table_id", e.tableOff), 404, "not_found"},
		{"too many guests", with("guests", 5), 422, "too_many_guests"},
		{"off grid", with("starts_at", e.tomorrow(19, 10)), 422, "outside_hours"},
		{"ends after close", with("starts_at", e.tomorrow(22, 30)), 422, "outside_hours"},
		{"in past", with("starts_at", e.tomorrow(19, 0).AddDate(0, 0, -2)), 422, "in_past"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.t = t
			e.wantError(e.call(http.MethodPost, "/api/bookings", 9, tt.body), tt.code, tt.errCode)
		})
	}
	e.t = t
	e.mustCall(http.MethodPost, "/api/bookings", 9, with("special_request", strings.Repeat("я", 300)), http.StatusCreated, nil)
	e.wantError(e.call(http.MethodPost, "/api/bookings", 0, ok), 401, "unauthorized")
}

func TestTimesAndTablesReflectBookings(t *testing.T) {
	e := newEnv(t)
	e.insertBooking(e.venueA, e.tableA4, 20, e.tomorrow(19, 0), "confirmed", nil)
	e.insertBooking(e.venueA, e.tableA6, 21, e.tomorrow(19, 0), "cancelled", nil)

	date := e.tomorrow(0, 0).Format(time.DateOnly)
	var times []struct {
		StartsAt   time.Time `json:"starts_at"`
		EndsAt     time.Time `json:"ends_at"`
		FreeTables int       `json:"free_tables"`
	}
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/venues/%d/times?date=%s&guests=4", e.venueA, date), 3, nil, 200, &times)
	free := map[string]int{}
	for _, tm := range times {
		free[tm.StartsAt.In(e.loc).Format("15:04")] = tm.FreeTables
	}
	if len(times) != 25 || times[0].StartsAt.In(e.loc).Format("15:04") != "10:00" || times[24].StartsAt.In(e.loc).Format("15:04") != "22:00" {
		t.Fatalf("got %d times from %v, want 25 from 10:00 to 22:00", len(times), times[0].StartsAt)
	}
	// T2 (4 seats) is confirmed 19:00-21:00; T3 (6) has only a cancelled booking; T1 (2) is too small.
	for slot, want := range map[string]int{"16:30": 2, "17:30": 1, "19:00": 1, "20:30": 1, "21:00": 2} {
		if free[slot] != want {
			t.Errorf("free_tables at %s = %d, want %d", slot, free[slot], want)
		}
	}
	var all []struct {
		FreeTables int `json:"free_tables"`
	}
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/venues/%d/times?date=%s", e.venueA, date), 3, nil, 200, &all)
	if all[18].FreeTables != 2 { // 19:00 with the default of 1 guest: T1 and T3
		t.Errorf("default guests: free_tables at 19:00 = %d, want 2", all[18].FreeTables)
	}

	var m struct {
		VenueID      int64     `json:"venue_id"`
		StartsAt     time.Time `json:"starts_at"`
		FloorPlanURL string    `json:"floor_plan_url"`
		Tables       []struct {
			Label  string  `json:"label"`
			X      float64 `json:"x"`
			Status string  `json:"status"`
		} `json:"tables"`
	}
	// "+" left unencoded on purpose: it arrives as a space and must still parse.
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/venues/%d/tables?starts_at=%s&guests=4", e.venueA, e.tomorrow(20, 0).Format(time.RFC3339)), 3, nil, 200, &m)
	got := map[string]string{}
	for _, tb := range m.Tables {
		got[tb.Label] = tb.Status
	}
	if len(m.Tables) != 3 || got["T1"] != "too_small" || got["T2"] != "taken" || got["T3"] != "free" {
		t.Errorf("tables = %+v", m.Tables)
	}
	if m.Tables[1].X != 20.5 || m.FloorPlanURL != "/api/static/plans/nomad-bistro.svg" || m.VenueID != e.venueA {
		t.Errorf("table map = %+v", m)
	}
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/venues/%d/tables?starts_at=%s&guests=4", e.venueA, e.tomorrow(20, 15).UTC().Format(time.RFC3339)), 3, nil), 422, "outside_hours")
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/venues/%d/tables?starts_at=2020-01-01T19:00:00Z&guests=4", e.venueA), 3, nil), 422, "outside_hours")
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/venues/%d/tables?starts_at=%s&guests=4", e.venueA, e.tomorrow(19, 0).AddDate(0, 0, -2).UTC().Format(time.RFC3339)), 3, nil), 422, "in_past")
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/venues/%d/tables?guests=4", e.venueA), 3, nil), 400, "bad_request")
}

func TestTimesDates(t *testing.T) {
	e := newEnv(t)
	path := func(d time.Time, extra string) string {
		return fmt.Sprintf("/api/venues/%d/times?date=%s%s", e.venueA, d.Format(time.DateOnly), extra)
	}
	now := time.Now()
	var today []struct {
		StartsAt time.Time `json:"starts_at"`
	}
	e.mustCall(http.MethodGet, path(now.In(e.loc), ""), 3, nil, 200, &today)
	for _, tm := range today {
		if !tm.StartsAt.After(now) {
			t.Errorf("today: %v is not in the future", tm.StartsAt)
		}
	}
	var past []any
	r := e.mustCall(http.MethodGet, path(now.In(e.loc).AddDate(0, 0, -1), ""), 3, nil, 200, &past)
	if len(past) != 0 || strings.TrimSpace(string(r.body)) != "[]" {
		t.Errorf("past date body = %s, want []", r.body)
	}
	e.mustCall(http.MethodGet, path(now.In(e.loc).AddDate(0, 0, 30), ""), 3, nil, 200, nil)
	e.wantError(e.call(http.MethodGet, path(now.In(e.loc).AddDate(0, 0, 31), ""), 3, nil), 422, "outside_hours")
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/venues/%d/times?date=2026-13-01", e.venueA), 3, nil), 400, "bad_request")
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/venues/%d/times", e.venueA), 3, nil), 400, "bad_request")
	e.wantError(e.call(http.MethodGet, path(now, "&guests=21"), 3, nil), 400, "bad_request")
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/venues/%d/times?date=%s", e.venueOff, now.Format(time.DateOnly)), 3, nil), 404, "not_found")
}

func TestVenuesListAndGet(t *testing.T) {
	e := newEnv(t)
	var list []struct {
		ID       int64  `json:"id"`
		Category string `json:"category"`
		OpensAt  string `json:"opens_at"`
		ClosesAt string `json:"closes_at"`
	}
	e.mustCall(http.MethodGet, "/api/venues", 3, nil, 200, &list)
	if len(list) != 2 || list[0].ID != e.venueA || list[1].ID != e.venueB {
		t.Fatalf("venues = %+v, want A then B (inactive hidden)", list)
	}
	if list[0].OpensAt != "10:00" || list[0].ClosesAt != "24:00" {
		t.Errorf("hours = %s-%s", list[0].OpensAt, list[0].ClosesAt)
	}
	e.mustCall(http.MethodGet, "/api/venues?category=cafe", 3, nil, 200, &list)
	if len(list) != 1 || list[0].ID != e.venueB {
		t.Errorf("cafes = %+v", list)
	}
	e.mustCall(http.MethodGet, "/api/venues?category=karaoke", 3, nil, 200, &list)
	if len(list) != 0 {
		t.Errorf("karaoke = %+v, want []", list)
	}
	e.wantError(e.call(http.MethodGet, "/api/venues?category=casino", 3, nil), 400, "bad_request")

	e.mustCall(http.MethodGet, fmt.Sprintf("/api/venues/%d", e.venueB), 3, nil, 200, nil)
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/venues/%d", e.venueOff), 3, nil), 404, "not_found")
	e.wantError(e.call(http.MethodGet, "/api/venues/999999", 3, nil), 404, "not_found")
	e.wantError(e.call(http.MethodGet, "/api/venues/abc", 3, nil), 400, "bad_request")
	e.wantError(e.call(http.MethodGet, "/api/venues", 0, nil), 401, "unauthorized")
}

func TestMyBookings(t *testing.T) {
	e := newEnv(t)
	past := time.Now().Add(-time.Minute)
	e.insertBooking(e.venueA, e.tableA4, 30, e.tomorrow(12, 0), "pending", &past) // overdue hold
	e.insertBooking(e.venueB, e.tableB4, 30, e.tomorrow(15, 0), "confirmed", nil)
	e.insertBooking(e.venueA, e.tableA6, 31, e.tomorrow(19, 0), "confirmed", nil) // someone else's
	var mine bookingJSON
	e.mustCall(http.MethodPost, "/api/bookings", 30, map[string]any{
		"table_id": e.tableA6, "starts_at": e.tomorrow(21, 0), "guests": 2,
	}, http.StatusCreated, &mine)

	var list []bookingJSON
	e.mustCall(http.MethodGet, "/api/bookings/my", 30, nil, 200, &list)
	if len(list) != 3 {
		t.Fatalf("got %d bookings, want 3", len(list))
	}
	want := []struct {
		hour   int
		status string
	}{{21, "pending"}, {15, "confirmed"}, {12, "expired"}}
	for i, w := range want {
		if list[i].StartsAt.In(e.loc).Hour() != w.hour || list[i].Status != w.status {
			t.Errorf("item %d = %s %s, want %02d:00 %s", i, list[i].StartsAt.In(e.loc).Format("15:04"), list[i].Status, w.hour, w.status)
		}
	}
	if list[0].ExpiresAt == nil || list[1].ExpiresAt != nil || list[2].ExpiresAt != nil {
		t.Errorf("expires_at should be set only while pending: %v %v %v", list[0].ExpiresAt, list[1].ExpiresAt, list[2].ExpiresAt)
	}
	if list[1].Venue.Name != "Venue B" || list[1].Table.Label != "B1" {
		t.Errorf("item 1 venue/table = %+v %+v", list[1].Venue, list[1].Table)
	}

	var empty []bookingJSON
	r := e.mustCall(http.MethodGet, "/api/bookings/my", 32, nil, 200, &empty)
	if strings.TrimSpace(string(r.body)) != "[]" {
		t.Errorf("no bookings body = %s, want []", r.body)
	}
}

func TestCancel(t *testing.T) {
	e := newEnv(t)
	var b bookingJSON
	e.mustCall(http.MethodPost, "/api/bookings", 40, map[string]any{
		"table_id": e.tableA4, "starts_at": e.tomorrow(19, 0), "guests": 2,
	}, http.StatusCreated, &b)
	path := fmt.Sprintf("/api/bookings/%d/cancel", b.ID)

	e.wantError(e.call(http.MethodPost, path, 41, nil), 404, "not_found")
	var c bookingJSON
	e.mustCall(http.MethodPost, path, 40, nil, 200, &c)
	if c.Status != "cancelled" || c.ID != b.ID || c.ExpiresAt != nil {
		t.Errorf("cancelled booking = %+v", c)
	}
	if len(e.notifier.cancelled) != 1 {
		t.Errorf("notifier.cancelled = %v", e.notifier.cancelled)
	}
	e.wantError(e.call(http.MethodPost, path, 40, nil), 409, "not_cancellable")

	// The table is free again.
	e.mustCall(http.MethodPost, "/api/bookings", 42, map[string]any{
		"table_id": e.tableA4, "starts_at": e.tomorrow(19, 0), "guests": 2,
	}, http.StatusCreated, nil)

	started := e.insertBooking(e.venueA, e.tableA6, 40, time.Now().Add(-30*time.Minute), "confirmed", nil)
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/bookings/%d/cancel", started), 40, nil), 409, "not_cancellable")
	past := time.Now().Add(-time.Minute)
	overdue := e.insertBooking(e.venueA, e.tableA2, 40, e.tomorrow(12, 0), "pending", &past)
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/bookings/%d/cancel", overdue), 40, nil), 409, "not_cancellable")
	e.wantError(e.call(http.MethodPost, "/api/bookings/999999/cancel", 40, nil), 404, "not_found")
}

func TestNotifierErrorDoesNotFailRequest(t *testing.T) {
	e := newEnv(t)
	e.notifier.err = errors.New("telegram is down")
	var b bookingJSON
	e.mustCall(http.MethodPost, "/api/bookings", 50, map[string]any{
		"table_id": e.tableA4, "starts_at": e.tomorrow(19, 0), "guests": 2,
	}, http.StatusCreated, &b)
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/bookings/%d/cancel", b.ID), 50, nil, 200, nil)
}
