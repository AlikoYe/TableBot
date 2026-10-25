package bot

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/TableBot/backend/internal/booking"
	"github.com/AlikoYe/TableBot/backend/internal/owner"
	"github.com/AlikoYe/TableBot/backend/internal/platform/dbtest"
)

// callbackEnv runs callbacks through the real owner service against a test DB.
type callbackEnv struct {
	db  *pgxpool.Pool
	fs  *fakeSender
	bot *Bot
}

func newCallbackEnv(t *testing.T) callbackEnv {
	t.Helper()
	db := dbtest.New(t)
	fs := &fakeSender{}
	log := slog.New(slog.DiscardHandler)
	n := testNotifier(fs, &bytes.Buffer{})
	svc := owner.NewService(owner.NewRepo(db), booking.NewRepo(db), n, log)
	return callbackEnv{db: db, fs: fs, bot: NewWithSender(fs, svc, "", log)}
}

// pendingBooking creates owner 1, guest 2, a venue and a live pending booking tomorrow.
func (e callbackEnv) pendingBooking(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := e.db.QueryRow(ctx, `
		WITH u AS (INSERT INTO users (telegram_id, name, username) VALUES (1, 'Owner', NULL), (2, 'Aigerim S.', 'aigerim')),
		     v AS (INSERT INTO venues (owner_id, name, category, opens_at, closes_at) VALUES (1, 'Nomad Bistro', 'restaurant', '12:00', '24:00') RETURNING id),
		     t AS (INSERT INTO venue_tables (venue_id, label, seats, x, y) SELECT id, 'T3', 4, 30, 20 FROM v RETURNING id, venue_id)
		SELECT 1`).Scan(new(int))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	err = e.db.QueryRow(ctx, `
		INSERT INTO bookings (venue_id, table_id, user_id, guests, during, status, expires_at)
		SELECT venue_id, id, 2, 4, tstzrange($1::timestamptz, $1::timestamptz + interval '2 hours', '[)'), 'pending', now() + interval '15 minutes'
		FROM venue_tables RETURNING id`, start).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (e callbackEnv) status(t *testing.T, id int64) string {
	t.Helper()
	var s string
	if err := e.db.QueryRow(context.Background(), `SELECT status FROM bookings WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func press(from int64, data string) *models.CallbackQuery {
	return &models.CallbackQuery{
		ID:   "cb1",
		From: models.User{ID: from},
		Data: data,
		Message: models.MaybeInaccessibleMessage{Message: &models.Message{
			ID: 77, Chat: models.Chat{ID: from}, Text: "🆕 New booking request\n🏠 Nomad Bistro · table T3",
		}},
	}
}

func TestCallbackOwnerConfirms(t *testing.T) {
	e := newCallbackEnv(t)
	id := e.pendingBooking(t)

	e.bot.HandleCallback(context.Background(), press(1, fmt.Sprintf("bk:c:%d", id)))

	if got := e.status(t, id); got != "confirmed" {
		t.Fatalf("status = %s, want confirmed", got)
	}
	if len(e.fs.answers) != 1 || e.fs.answers[0].ShowAlert || e.fs.answers[0].Text != textOutcomeOK {
		t.Errorf("answers = %+v", e.fs.answers)
	}
	if len(e.fs.edits) != 1 || e.fs.edits[0].chatID != 1 || !strings.Contains(e.fs.edits[0].text, textOutcomeOK) || e.fs.edits[0].markup != nil {
		t.Errorf("owner message edit = %+v (want outcome text, no buttons)", e.fs.edits)
	}
	if len(e.fs.sends) != 1 || e.fs.sends[0].chatID != 2 || !strings.Contains(e.fs.sends[0].text, "confirmed") {
		t.Errorf("guest message = %+v", e.fs.sends)
	}

	// Pressing again: no longer pending.
	e.bot.HandleCallback(context.Background(), press(1, fmt.Sprintf("bk:r:%d", id)))
	if last := e.fs.answers[len(e.fs.answers)-1]; !last.ShowAlert || last.Text != textNotPending {
		t.Errorf("second press answer = %+v", last)
	}
	if got := e.status(t, id); got != "confirmed" {
		t.Fatalf("status after second press = %s", got)
	}
}

func TestCallbackOwnerRejects(t *testing.T) {
	e := newCallbackEnv(t)
	id := e.pendingBooking(t)
	e.bot.HandleCallback(context.Background(), press(1, fmt.Sprintf("bk:r:%d", id)))
	if got := e.status(t, id); got != "rejected" {
		t.Fatalf("status = %s, want rejected", got)
	}
	if len(e.fs.sends) != 1 || !strings.Contains(e.fs.sends[0].text, "can't take your booking") {
		t.Errorf("guest message = %+v", e.fs.sends)
	}
}

func TestCallbackFromNonOwnerIsRefused(t *testing.T) {
	e := newCallbackEnv(t)
	id := e.pendingBooking(t)
	e.bot.HandleCallback(context.Background(), press(2, fmt.Sprintf("bk:c:%d", id)))

	if got := e.status(t, id); got != "pending" {
		t.Fatalf("status = %s, want pending", got)
	}
	if len(e.fs.answers) != 1 || !e.fs.answers[0].ShowAlert || e.fs.answers[0].Text != textForbidden {
		t.Errorf("answers = %+v", e.fs.answers)
	}
	if len(e.fs.edits) != 1 || e.fs.edits[0].markup != nil {
		t.Errorf("buttons should be removed: %+v", e.fs.edits)
	}
	if len(e.fs.sends) != 0 {
		t.Errorf("nobody should be messaged: %+v", e.fs.sends)
	}
}

func TestCallbackUnknownBooking(t *testing.T) {
	e := newCallbackEnv(t)
	e.bot.HandleCallback(context.Background(), press(1, "bk:c:999999"))
	if len(e.fs.answers) != 1 || e.fs.answers[0].Text != textNotFound {
		t.Errorf("answers = %+v", e.fs.answers)
	}
}
