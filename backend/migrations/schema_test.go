package migrations_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/TableBot/backend/internal/platform/dbtest"
)

const (
	sqlstateExclusion = "23P01"
	sqlstateFK        = "23503"
	sqlstateCheck     = "23514"
)

// Fixed future evening in Almaty, written as tstzrange literals.
const (
	at1900to2100 = "[2030-06-01 19:00+05, 2030-06-01 21:00+05)"
	at2000to2200 = "[2030-06-01 20:00+05, 2030-06-01 22:00+05)"
	at2100to2300 = "[2030-06-01 21:00+05, 2030-06-01 23:00+05)"
)

type fixture struct {
	db                       *pgxpool.Pool
	venueA, venueB           int64
	tableA1, tableA2, tableB int64
}

// Users 1 (owner), 2 and 3; venue A with tables A1/A2; venue B with one table.
func setup(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	f := fixture{db: dbtest.New(t)}

	mustExec(t, f.db, `INSERT INTO users (telegram_id, name) VALUES (1, 'Owner'), (2, 'Ann'), (3, 'Bob')`)
	for _, v := range []struct {
		name string
		id   *int64
	}{{"Venue A", &f.venueA}, {"Venue B", &f.venueB}} {
		err := f.db.QueryRow(ctx, `
			INSERT INTO venues (owner_id, name, category, opens_at, closes_at)
			VALUES (1, $1, 'restaurant', '12:00', '24:00') RETURNING id`, v.name).Scan(v.id)
		if err != nil {
			t.Fatalf("insert venue: %v", err)
		}
	}
	for _, tb := range []struct {
		venue int64
		label string
		id    *int64
	}{{f.venueA, "A1", &f.tableA1}, {f.venueA, "A2", &f.tableA2}, {f.venueB, "B1", &f.tableB}} {
		err := f.db.QueryRow(ctx, `
			INSERT INTO venue_tables (venue_id, label, seats, x, y)
			VALUES ($1, $2, 4, 50, 50) RETURNING id`, tb.venue, tb.label).Scan(tb.id)
		if err != nil {
			t.Fatalf("insert table: %v", err)
		}
	}
	return f
}

func (f fixture) book(venueID, tableID, userID int64, during, status string) error {
	_, err := f.db.Exec(context.Background(), `
		INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status, expires_at)
		VALUES ($1, $2, $3, 2, $4::tstzrange, $5,
		        CASE WHEN $5 = 'pending' THEN now() + interval '15 minutes' END)`,
		venueID, tableID, userID, during, status)
	return err
}

func TestTableOverlapIsRejected(t *testing.T) {
	for _, first := range []string{"pending", "confirmed"} {
		for _, second := range []string{"pending", "confirmed"} {
			t.Run(first+"_then_"+second, func(t *testing.T) {
				f := setup(t)
				mustOK(t, f.book(f.venueA, f.tableA1, 2, at1900to2100, first))
				err := f.book(f.venueA, f.tableA1, 3, at2000to2200, second)
				assertPgError(t, err, sqlstateExclusion, "bookings_no_table_overlap")
			})
		}
	}
}

func TestAdjacentRangesOnSameTableAreAllowed(t *testing.T) {
	f := setup(t)
	mustOK(t, f.book(f.venueA, f.tableA1, 2, at1900to2100, "confirmed"))
	mustOK(t, f.book(f.venueA, f.tableA1, 3, at2100to2300, "pending"))
}

func TestOverlapOnDifferentTablesIsAllowed(t *testing.T) {
	f := setup(t)
	mustOK(t, f.book(f.venueA, f.tableA1, 2, at1900to2100, "confirmed"))
	mustOK(t, f.book(f.venueA, f.tableA2, 3, at2000to2200, "confirmed"))
}

func TestInactiveBookingsDoNotBlock(t *testing.T) {
	for _, status := range []string{"rejected", "cancelled", "expired"} {
		t.Run(status, func(t *testing.T) {
			f := setup(t)
			mustOK(t, f.book(f.venueA, f.tableA1, 2, at1900to2100, "pending"))
			mustExec(t, f.db, `UPDATE bookings SET status = $1, decided_at = now()`, status)

			// Same table by another user, and the same user elsewhere.
			mustOK(t, f.book(f.venueA, f.tableA1, 3, at2000to2200, "pending"))
			mustOK(t, f.book(f.venueB, f.tableB, 2, at2000to2200, "pending"))
		})
	}
}

func TestUserCannotHoldOverlappingBookingsAcrossVenues(t *testing.T) {
	f := setup(t)
	mustOK(t, f.book(f.venueA, f.tableA1, 2, at1900to2100, "confirmed"))
	err := f.book(f.venueB, f.tableB, 2, at2000to2200, "pending")
	assertPgError(t, err, sqlstateExclusion, "bookings_no_user_overlap")

	// Back-to-back is fine.
	mustOK(t, f.book(f.venueB, f.tableB, 2, at2100to2300, "pending"))
}

func TestTableMustBelongToVenue(t *testing.T) {
	f := setup(t)
	err := f.book(f.venueB, f.tableA1, 2, at1900to2100, "pending")
	assertPgError(t, err, sqlstateFK, "bookings_table_venue_fkey")
}

func TestExpireBeforeInsert(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	mustOK(t, f.book(f.venueA, f.tableA1, 2, at1900to2100, "pending"))
	mustExec(t, f.db, `UPDATE bookings SET expires_at = now() - interval '1 minute'`)

	// An overdue pending booking still blocks until it is flipped to expired.
	assertPgError(t, f.book(f.venueA, f.tableA1, 3, at1900to2100, "pending"), sqlstateExclusion, "bookings_no_table_overlap")

	tx, err := f.db.Begin(ctx)
	mustOK(t, err)
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE bookings SET status = 'expired', updated_at = now()
		WHERE status = 'pending' AND expires_at < now() AND (table_id = $1 OR user_id = $2)`,
		f.tableA1, int64(3))
	mustOK(t, err)
	if tag.RowsAffected() != 1 {
		t.Fatalf("expired %d rows, want 1", tag.RowsAffected())
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status, expires_at)
		VALUES ($1, $2, 3, 2, $3::tstzrange, 'pending', now() + interval '15 minutes')`,
		f.venueA, f.tableA1, at1900to2100)
	mustOK(t, err)
	mustOK(t, tx.Commit(ctx))
}

func TestCheckConstraints(t *testing.T) {
	f := setup(t)
	tests := []struct {
		name, sql string
	}{
		{"pending without expires_at", fmt.Sprintf(`INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status)
			VALUES (%d, %d, 2, 2, '%s', 'pending')`, f.venueA, f.tableA1, at1900to2100)},
		{"inclusive upper bound", fmt.Sprintf(`INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status)
			VALUES (%d, %d, 2, 2, '[2030-06-01 19:00+05, 2030-06-01 21:00+05]', 'confirmed')`, f.venueA, f.tableA1)},
		{"empty range", fmt.Sprintf(`INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status)
			VALUES (%d, %d, 2, 2, 'empty', 'confirmed')`, f.venueA, f.tableA1)},
		{"too many guests", fmt.Sprintf(`INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status)
			VALUES (%d, %d, 2, 21, '%s', 'confirmed')`, f.venueA, f.tableA1, at1900to2100)},
		{"unknown status", fmt.Sprintf(`INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status)
			VALUES (%d, %d, 2, 2, '%s', 'completed')`, f.venueA, f.tableA1, at1900to2100)},
		{"special request too long", fmt.Sprintf(`INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status, special_request)
			VALUES (%d, %d, 2, 2, '%s', 'confirmed', repeat('x', 301))`, f.venueA, f.tableA1, at1900to2100)},
		{"closes before opens", `INSERT INTO venues (name, category, opens_at, closes_at) VALUES ('X', 'cafe', '22:00', '02:00')`},
		{"unknown category", `INSERT INTO venues (name, category, opens_at, closes_at) VALUES ('X', 'casino', '10:00', '20:00')`},
		{"staff role is gone", `INSERT INTO users (telegram_id, role) VALUES (9, 'staff')`},
		{"x outside plan", fmt.Sprintf(`INSERT INTO venue_tables (venue_id, label, seats, x, y) VALUES (%d, 'Z', 2, 100.5, 10)`, f.venueA)},
		{"zero seats", fmt.Sprintf(`INSERT INTO venue_tables (venue_id, label, seats, x, y) VALUES (%d, 'Z', 0, 10, 10)`, f.venueA)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.db.Exec(context.Background(), tt.sql)
			assertPgError(t, err, sqlstateCheck, "")
		})
	}

	// 24:00 closing time is allowed.
	mustExec(t, f.db, `INSERT INTO venues (name, category, opens_at, closes_at) VALUES ('Late', 'bar', '18:00', '24:00')`)
}

func TestMigrationsUpDownUp(t *testing.T) {
	db, provider := dbtest.Reset(t)
	ctx := context.Background()
	tables := []string{"users", "venues", "venue_tables", "bookings"}

	exists := func(name string) bool {
		var ok bool
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&ok); err != nil {
			t.Fatalf("to_regclass(%s): %v", name, err)
		}
		return ok
	}
	check := func(want bool) {
		t.Helper()
		for _, name := range tables {
			if got := exists(name); got != want {
				t.Fatalf("table %s exists = %v, want %v", name, got, want)
			}
		}
	}

	_, err := provider.Up(ctx)
	mustOK(t, err)
	check(true)
	_, err = provider.DownTo(ctx, 0)
	mustOK(t, err)
	check(false)
	_, err = provider.Up(ctx)
	mustOK(t, err)
	check(true)
}

func assertPgError(t *testing.T, err error, code, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %v, want PostgreSQL error %s", err, code)
	}
	if pgErr.Code != code {
		t.Fatalf("SQLSTATE = %s (%s), want %s", pgErr.Code, pgErr.Message, code)
	}
	if constraint != "" && pgErr.ConstraintName != constraint {
		t.Fatalf("constraint = %q, want %q", pgErr.ConstraintName, constraint)
	}
}

func mustExec(t *testing.T, db *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := db.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
