package notify

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/TableBot/backend/internal/booking"
	"github.com/AlikoYe/TableBot/backend/internal/platform/dbtest"
)

type fakeNotifier struct {
	mu       sync.Mutex
	expired  []int64
	reminded []int64
	failIDs  []int64
}

func (f *fakeNotifier) record(list *[]int64, b booking.Booking) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	*list = append(*list, b.ID)
	if slices.Contains(f.failIDs, b.ID) {
		return errors.New("telegram is down")
	}
	return nil
}

func (f *fakeNotifier) BookingExpired(_ context.Context, b booking.Booking) error {
	return f.record(&f.expired, b)
}

func (f *fakeNotifier) BookingReminder(_ context.Context, b booking.Booking) error {
	return f.record(&f.reminded, b)
}

type workerEnv struct {
	db      *pgxpool.Pool
	n       *fakeNotifier
	w       *Worker
	tableID int64
	venueID int64
	nextUsr int64
}

func newWorkerEnv(t *testing.T) *workerEnv {
	t.Helper()
	db := dbtest.New(t)
	e := &workerEnv{db: db, n: &fakeNotifier{}, nextUsr: 100}
	e.w = NewWorker(db, booking.NewRepo(db), e.n, time.Minute, 2*time.Hour, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	if err := db.QueryRow(ctx, `INSERT INTO venues (name, category, opens_at, closes_at) VALUES ('V', 'bar', '00:00', '24:00') RETURNING id`).Scan(&e.venueID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO venue_tables (venue_id, label, seats, x, y) VALUES ($1, 'T1', 4, 10, 10) RETURNING id`, e.venueID).Scan(&e.tableID); err != nil {
		t.Fatal(err)
	}
	return e
}

// add inserts a booking for a fresh user on the one table; starts are spaced so they never overlap.
func (e *workerEnv) add(t *testing.T, startIn time.Duration, status string, expiresIn *time.Duration) int64 {
	t.Helper()
	ctx := context.Background()
	e.nextUsr++
	if _, err := e.db.Exec(ctx, `INSERT INTO users (telegram_id) VALUES ($1)`, e.nextUsr); err != nil {
		t.Fatal(err)
	}
	var expires *time.Time
	if expiresIn != nil {
		x := time.Now().Add(*expiresIn)
		expires = &x
	}
	start := time.Now().Add(startIn)
	var id int64
	err := e.db.QueryRow(ctx, `
		INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status, expires_at)
		VALUES ($1, $2, $3, 2, tstzrange($4::timestamptz, $4::timestamptz + interval '10 minutes', '[)'), $5, $6) RETURNING id`,
		e.venueID, e.tableID, e.nextUsr, start, status, expires).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *workerEnv) status(t *testing.T, id int64) string {
	t.Helper()
	var s string
	if err := e.db.QueryRow(context.Background(), `SELECT status FROM bookings WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func dur(d time.Duration) *time.Duration { return &d }

func TestWorkerExpiresOverdueHoldsOnce(t *testing.T) {
	e := newWorkerEnv(t)
	overdue := e.add(t, 5*time.Hour, "pending", dur(-time.Minute))
	live := e.add(t, 6*time.Hour, "pending", dur(10*time.Minute))

	res := e.w.RunOnce(context.Background())
	if res.Expired != 1 || !slices.Equal(e.n.expired, []int64{overdue}) {
		t.Fatalf("first run: %+v, notified %v; want 1 expired = [%d]", res, e.n.expired, overdue)
	}
	if e.status(t, overdue) != "expired" || e.status(t, live) != "pending" {
		t.Fatalf("statuses: overdue=%s live=%s", e.status(t, overdue), e.status(t, live))
	}
	if res := e.w.RunOnce(context.Background()); res.Expired != 0 || len(e.n.expired) != 1 {
		t.Fatalf("second run: %+v, notified %v; want nothing new", res, e.n.expired)
	}
}

func TestWorkerRemindsOnceWithinWindow(t *testing.T) {
	e := newWorkerEnv(t)
	soon := e.add(t, time.Hour, "confirmed", nil)
	e.add(t, 3*time.Hour, "confirmed", nil)                  // too far ahead
	e.add(t, 90*time.Minute, "cancelled", nil)               // cancelled
	e.add(t, 100*time.Minute, "pending", dur(5*time.Minute)) // not confirmed
	e.add(t, -30*time.Minute, "confirmed", nil)              // already started

	res := e.w.RunOnce(context.Background())
	if res.Reminded != 1 || !slices.Equal(e.n.reminded, []int64{soon}) {
		t.Fatalf("first run: %+v, reminded %v; want [%d]", res, e.n.reminded, soon)
	}
	if res := e.w.RunOnce(context.Background()); res.Reminded != 0 || len(e.n.reminded) != 1 {
		t.Fatalf("second run: %+v, reminded %v; want nothing new", res, e.n.reminded)
	}
}

func TestWorkerOneFailedSendDoesNotStopOthers(t *testing.T) {
	e := newWorkerEnv(t)
	a := e.add(t, 5*time.Hour, "pending", dur(-time.Minute))
	b := e.add(t, 6*time.Hour, "pending", dur(-time.Minute))
	c := e.add(t, time.Hour, "confirmed", nil)
	e.n.failIDs = []int64{a}

	res := e.w.RunOnce(context.Background())
	if res.Expired != 2 || res.Reminded != 1 || res.Failed != 1 {
		t.Fatalf("result = %+v, want 2 expired, 1 reminded, 1 failed", res)
	}
	slices.Sort(e.n.expired)
	if !slices.Equal(e.n.expired, []int64{a, b}) || !slices.Equal(e.n.reminded, []int64{c}) {
		t.Fatalf("expired %v reminded %v", e.n.expired, e.n.reminded)
	}
}
