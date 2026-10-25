// Package notify runs background jobs that message users, such as booking
// reminders.
package notify

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlikoYe/TableBot/backend/internal/bot"
)

// Worker periodically looks for bookings that need a reminder.
type Worker struct {
	interval time.Duration
	notifier bot.Notifier
	log      *slog.Logger
}

func NewWorker(interval time.Duration, notifier bot.Notifier, log *slog.Logger) *Worker {
	return &Worker{interval: interval, notifier: notifier, log: log}
}

// Run ticks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	w.log.Info("reminder worker started", slog.Duration("interval", w.interval))
	t := time.NewTicker(w.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("reminder worker stopped")
			return nil
		case <-t.C:
			w.tick(ctx)
		}
	}
}

// tick is one worker pass.
// TODO: flip overdue pending bookings to 'expired'; send reminders for confirmed
// bookings starting soon with reminder_sent = false, then mark reminder_sent.
func (w *Worker) tick(ctx context.Context) {
	w.log.DebugContext(ctx, "reminder tick")
}
