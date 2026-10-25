// Package bot runs the Telegram bot (long polling) and exposes a Notifier
// for sending messages to users from other parts of the app.
package bot

import (
	"context"
	"fmt"
	"log/slog"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Notifier sends a plain-text message to a Telegram user.
type Notifier interface {
	Notify(ctx context.Context, telegramID int64, text string) error
}

// Bot wraps the Telegram client. It implements Notifier.
type Bot struct {
	api       *tgbot.Bot
	webAppURL string
	log       *slog.Logger
}

// New creates the bot and verifies the token with getMe.
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
	api.RegisterHandler(tgbot.HandlerTypeMessageText, "start", tgbot.MatchTypeCommandStartOnly, b.handleStart)

	b.api = api
	return b, nil
}

// Run long-polls for updates until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	b.log.Info("telegram bot started (long polling)")
	b.api.Start(ctx)
	b.log.Info("telegram bot stopped")
	return nil
}

func (b *Bot) Notify(ctx context.Context, telegramID int64, text string) error {
	_, err := b.api.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: telegramID, Text: text})
	return err
}

func (b *Bot) handleStart(ctx context.Context, api *tgbot.Bot, update *models.Update) {
	params := &tgbot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Welcome to TableBot! Book a table at restaurants, cafes, billiards and more in a few taps.",
	}
	if b.webAppURL != "" {
		params.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{
				{Text: "Book a table", WebApp: &models.WebAppInfo{URL: b.webAppURL}},
			}},
		}
	}
	if _, err := api.SendMessage(ctx, params); err != nil {
		b.log.ErrorContext(ctx, "reply to /start", slog.Any("err", err))
	}
}

func (b *Bot) defaultHandler(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	// TODO: handle other commands / callbacks.
	b.log.DebugContext(ctx, "unhandled telegram update", slog.Int64("update_id", update.ID))
}

// LogNotifier is used when no bot token is configured (dev only): it logs
// messages instead of sending them.
type LogNotifier struct {
	Log *slog.Logger
}

func (n LogNotifier) Notify(ctx context.Context, telegramID int64, text string) error {
	n.Log.InfoContext(ctx, "notify (bot disabled)", slog.Int64("telegram_id", telegramID), slog.String("text", text))
	return nil
}
