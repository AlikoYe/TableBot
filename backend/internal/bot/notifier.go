package bot

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/AlikoYe/TableBot/backend/internal/booking"
	"github.com/AlikoYe/TableBot/backend/internal/bookingstatus"
)

// Callback data for the owner's buttons: "bk:c:<id>" confirms, "bk:r:<id>" rejects.
const (
	callbackPrefix  = "bk:"
	callbackConfirm = "bk:c:"
	callbackReject  = "bk:r:"
)

// Notifier sends booking messages through a Sender. Send errors are logged and swallowed,
// so a blocked bot never fails a request.
type Notifier struct {
	send      Sender
	webAppURL string
	log       *slog.Logger
	now       func() time.Time
}

func NewNotifier(send Sender, webAppURL string, log *slog.Logger) *Notifier {
	return &Notifier{send: send, webAppURL: webAppURL, log: log, now: time.Now}
}

// BookingCreated asks the venue owner to confirm or reject.
func (n *Notifier) BookingCreated(ctx context.Context, b booking.Booking) error {
	id := strconv.FormatInt(b.ID, 10)
	rows := [][]models.InlineKeyboardButton{{
		{Text: textConfirm, CallbackData: callbackConfirm + id},
		{Text: textReject, CallbackData: callbackReject + id},
	}}
	if n.webAppURL != "" {
		rows = append(rows, []models.InlineKeyboardButton{{Text: textOpenApp, WebApp: &models.WebAppInfo{URL: n.webAppURL}}})
	}
	n.sendTo(ctx, b, b.OwnerID, "booking_created", textNewBooking(b, n.now()), &models.InlineKeyboardMarkup{InlineKeyboard: rows})
	return nil
}

// BookingCancelled tells the owner the guest cancelled.
func (n *Notifier) BookingCancelled(ctx context.Context, b booking.Booking) error {
	n.sendTo(ctx, b, b.OwnerID, "booking_cancelled", textCancelledForOwner(b), nil)
	return nil
}

// BookingDecided tells the guest the owner confirmed or rejected.
func (n *Notifier) BookingDecided(ctx context.Context, b booking.Booking) error {
	text := textRejectedForGuest(b)
	if b.Status == bookingstatus.Confirmed {
		text = textConfirmedForGuest(b)
	}
	n.sendTo(ctx, b, b.UserID, "booking_decided", text, nil)
	return nil
}

// BookingExpired tells the guest the owner did not answer in time.
func (n *Notifier) BookingExpired(ctx context.Context, b booking.Booking) error {
	n.sendTo(ctx, b, b.UserID, "booking_expired", textExpiredForGuest(b), nil)
	return nil
}

// BookingReminder reminds the guest of an upcoming confirmed booking.
func (n *Notifier) BookingReminder(ctx context.Context, b booking.Booking) error {
	n.sendTo(ctx, b, b.UserID, "booking_reminder", textReminderForGuest(b), nil)
	return nil
}

func (n *Notifier) sendTo(ctx context.Context, b booking.Booking, chatID int64, event, text string, markup models.ReplyMarkup) {
	if chatID == 0 {
		n.log.InfoContext(ctx, "no recipient for notification", slog.String("event", event), slog.Int64("booking_id", b.ID))
		return
	}
	params := &tgbot.SendMessageParams{ChatID: chatID, Text: text}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	if _, err := n.send.SendMessage(ctx, params); err != nil {
		n.log.WarnContext(ctx, "telegram send failed",
			slog.String("event", event), slog.Int64("booking_id", b.ID), slog.Int64("chat_id", chatID), slog.Any("err", err))
	}
}
