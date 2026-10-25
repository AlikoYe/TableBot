// Package bot runs the Telegram bot (long polling): /start, the menu button, booking
// notifications and the owner's confirm/reject buttons.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/AlikoYe/TableBot/backend/internal/booking"
	"github.com/AlikoYe/TableBot/backend/internal/owner"
)

// Decider confirms or rejects a booking as the acting Telegram user (owner.Service).
type Decider interface {
	Decide(ctx context.Context, actorID, bookingID int64, confirm bool) (booking.Booking, error)
}

// Bot wraps the Telegram client and its update handlers.
type Bot struct {
	api       *tgbot.Bot
	send      Sender
	decider   Decider
	webAppURL string
	log       *slog.Logger
}

// New creates the Telegram client. Call Handle before Run.
func New(token, webAppURL string, log *slog.Logger) (*Bot, error) {
	b := &Bot{webAppURL: webAppURL, log: log}
	api, err := tgbot.New(token,
		tgbot.WithDefaultHandler(b.defaultHandler),
		tgbot.WithErrorsHandler(func(err error) {
			log.Warn("telegram bot error", slog.Any("err", err))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}
	b.api = api
	b.send = api
	return b, nil
}

// NewWithSender builds a Bot without a Telegram client, for tests.
func NewWithSender(send Sender, decider Decider, webAppURL string, log *slog.Logger) *Bot {
	return &Bot{send: send, decider: decider, webAppURL: webAppURL, log: log}
}

// Sender returns the client used to send messages.
func (b *Bot) Sender() Sender { return b.send }

// Handle registers /start and the confirm/reject callbacks.
func (b *Bot) Handle(decider Decider) {
	b.decider = decider
	b.api.RegisterHandler(tgbot.HandlerTypeMessageText, "start", tgbot.MatchTypeCommandStartOnly, b.handleStart)
	b.api.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, callbackPrefix, tgbot.MatchTypePrefix, b.handleCallback)
}

// Run sets the menu button and long-polls for updates until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	if b.webAppURL != "" {
		_, err := b.api.SetChatMenuButton(ctx, &tgbot.SetChatMenuButtonParams{
			MenuButton: models.MenuButtonWebApp{
				Type:   models.MenuButtonTypeWebApp,
				Text:   textMenuButton,
				WebApp: models.WebAppInfo{URL: b.webAppURL},
			},
		})
		if err != nil {
			b.log.WarnContext(ctx, "set chat menu button", slog.Any("err", err))
		}
	}
	b.log.Info("telegram bot started (long polling)")
	b.api.Start(ctx)
	b.log.Info("telegram bot stopped")
	return nil
}

func (b *Bot) handleStart(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	params := &tgbot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: textWelcome}
	if b.webAppURL != "" {
		params.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{
				{Text: textOpenApp, WebApp: &models.WebAppInfo{URL: b.webAppURL}},
			}},
		}
	}
	if _, err := b.send.SendMessage(ctx, params); err != nil {
		b.log.WarnContext(ctx, "reply to /start", slog.Any("err", err))
	}
}

// HandleCallback processes "bk:c:<id>" / "bk:r:<id>" from the owner's buttons.
func (b *Bot) HandleCallback(ctx context.Context, q *models.CallbackQuery) {
	answer := func(text string, alert bool) {
		_, err := b.send.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{CallbackQueryID: q.ID, Text: text, ShowAlert: alert})
		if err != nil {
			b.log.WarnContext(ctx, "answer callback", slog.Any("err", err))
		}
	}

	confirm := strings.HasPrefix(q.Data, callbackConfirm)
	raw := strings.TrimPrefix(strings.TrimPrefix(q.Data, callbackConfirm), callbackReject)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || (!confirm && !strings.HasPrefix(q.Data, callbackReject)) {
		answer(textTryAgain, true)
		return
	}

	bk, err := b.decider.Decide(ctx, q.From.ID, id, confirm)
	switch {
	case err == nil:
		outcome := textOutcomeNo
		if confirm {
			outcome = textOutcomeOK
		}
		answer(outcome, false)
		b.editOwnerMessage(ctx, q, id, textDecidedForOwner(bk, confirm))
	case errors.Is(err, owner.ErrNotPending):
		answer(textNotPending, true)
		b.editOwnerMessage(ctx, q, id, originalText(q)+"\n\n"+textNoLongerOpen)
	case errors.Is(err, owner.ErrForbidden):
		answer(textForbidden, true)
		b.editOwnerMessage(ctx, q, id, originalText(q))
	case errors.Is(err, owner.ErrNotFound):
		answer(textNotFound, true)
		b.editOwnerMessage(ctx, q, id, originalText(q))
	default:
		b.log.ErrorContext(ctx, "decide from bot", slog.Int64("booking_id", id), slog.Any("err", err))
		answer(textTryAgain, true)
	}
}

func (b *Bot) handleCallback(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.CallbackQuery != nil {
		b.HandleCallback(ctx, update.CallbackQuery)
	}
}

// editOwnerMessage replaces the message text; without reply_markup Telegram drops the buttons.
func (b *Bot) editOwnerMessage(ctx context.Context, q *models.CallbackQuery, bookingID int64, text string) {
	msg := q.Message.Message
	if msg == nil || text == "" {
		return
	}
	_, err := b.send.EditMessageText(ctx, &tgbot.EditMessageTextParams{ChatID: msg.Chat.ID, MessageID: msg.ID, Text: text})
	if err != nil {
		b.log.WarnContext(ctx, "edit owner message", slog.Int64("booking_id", bookingID), slog.Any("err", err))
	}
}

func originalText(q *models.CallbackQuery) string {
	if q.Message.Message == nil {
		return ""
	}
	return q.Message.Message.Text
}

func (b *Bot) defaultHandler(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	b.log.DebugContext(ctx, "unhandled telegram update", slog.Int64("update_id", update.ID))
}
