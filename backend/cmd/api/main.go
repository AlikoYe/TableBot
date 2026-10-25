// Command api runs the TableBot HTTP API, the Telegram bot and the reminder
// worker in one process, with graceful shutdown on SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/AlikoYe/TableBot/backend/internal/bot"
	"github.com/AlikoYe/TableBot/backend/internal/notify"
	"github.com/AlikoYe/TableBot/backend/internal/platform/config"
	"github.com/AlikoYe/TableBot/backend/internal/platform/database"
	"github.com/AlikoYe/TableBot/backend/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := logging.New(cfg.LogLevel, cfg.IsDev())
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// The bot is optional in dev: without a token, messages are only logged.
	var (
		tg     *bot.Bot
		sender bot.Sender = bot.LogSender{Log: log}
	)
	if cfg.BotToken != "" {
		tg, err = bot.New(cfg.BotToken, cfg.WebAppURL, log)
		if err != nil {
			return err
		}
		sender = tg.Sender()
	} else {
		log.Warn("BOT_TOKEN is empty: telegram bot disabled, messages are logged and only dev auth works")
	}
	notifier := bot.NewNotifier(sender, cfg.WebAppURL, log)
	svc := newServices(cfg, log, pool, notifier)
	if tg != nil {
		tg.Handle(svc.owner)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           newRouter(cfg, log, pool, svc),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	worker := notify.NewWorker(pool, svc.bookingRepo, notifier, cfg.ReminderInterval, cfg.ReminderBefore, log)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Info("http server listening", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.AppEnv))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})
	if tg != nil {
		g.Go(func() error { return tg.Run(gctx) })
	}
	g.Go(func() error { return worker.Run(gctx) })

	if err := g.Wait(); err != nil {
		return err
	}
	log.Info("stopped cleanly")
	return nil
}
