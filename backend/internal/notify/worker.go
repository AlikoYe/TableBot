// Package notify runs the background worker: it expires overdue holds and sends
// reminders before confirmed bookings.
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/TableBot/backend/internal/booking"
	"github.com/AlikoYe/TableBot/backend/internal/bookingstatus"
)

// Notifier messages the guest. Implemented by bot.Notifier.
type Notifier interface {
	BookingExpired(ctx context.Context, b booking.Booking) error
	BookingReminder(ctx context.Context, b booking.Booking) error
}

type Worker struct {
	db             *pgxpool.Pool
	bookings       *booking.Repo
	notifier       Notifier
	interval       time.Duration
	reminderBefore time.Duration
	log            *slog.Logger
}

func NewWorker(db *pgxpool.Pool, bookings *booking.Repo, notifier Notifier, interval, reminderBefore time.Duration, log *slog.Logger) *Worker {
	return &Worker{db: db, bookings: bookings, notifier: notifier, interval: interval, reminderBefore: reminderBefore, log: log}
}

// Run ticks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	w.log.Info("worker started", slog.Duration("interval", w.interval), slog.Duration("reminder_before", w.reminderBefore))
	t := time.NewTicker(w.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("worker stopped")
			return nil
		case <-t.C:
			w.RunOnce(ctx)
		}
	}
}

// Result counts what one run did.
type Result struct {
	Expired, Reminded, Failed int
}

// RunOnce expires overdue holds and sends due reminders. Each step and each send is independent.
func (w *Worker) RunOnce(ctx context.Context) Result {
	var res Result

	ids, err := w.claim(ctx, bookingstatus.ExpireAllOverdueSQL)
	if err != nil {
		w.log.ErrorContext(ctx, "expire overdue bookings", slog.Any("err", err))
	}
	res.Expired = len(ids)
	res.Failed += w.notifyEach(ctx, ids, "expired", w.notifier.BookingExpired)

	ids, err = w.claim(ctx, bookingstatus.ClaimRemindersSQL, w.reminderBefore.Seconds())
	if err != nil {
		w.log.ErrorContext(ctx, "claim reminders", slog.Any("err", err))
	}
	res.Reminded = len(ids)
	res.Failed += w.notifyEach(ctx, ids, "reminder", w.notifier.BookingReminder)

	level := slog.LevelDebug
	if res.Expired+res.Reminded+res.Failed > 0 {
		level = slog.LevelInfo
	}
	w.log.Log(ctx, level, "worker run", slog.Int("expired", res.Expired), slog.Int("reminded", res.Reminded), slog.Int("failed", res.Failed))
	return res
}

func (w *Worker) claim(ctx context.Context, sql string, args ...any) ([]int64, error) {
	rows, err := w.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("scan ids: %w", err)
	}
	return ids, nil
}

func (w *Worker) notifyEach(ctx context.Context, ids []int64, event string, send func(context.Context, booking.Booking) error) (failed int) {
	for _, id := range ids {
		b, err := w.bookings.Get(ctx, id)
		if err == nil {
			err = send(ctx, b)
		}
		if err != nil {
			failed++
			w.log.WarnContext(ctx, "worker notification failed", slog.String("event", event), slog.Int64("booking_id", id), slog.Any("err", err))
		}
	}
	return failed
}
