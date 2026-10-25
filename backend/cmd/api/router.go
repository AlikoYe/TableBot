package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/TableBot/backend/internal/auth"
	"github.com/AlikoYe/TableBot/backend/internal/booking"
	"github.com/AlikoYe/TableBot/backend/internal/owner"
	"github.com/AlikoYe/TableBot/backend/internal/platform/config"
	"github.com/AlikoYe/TableBot/backend/internal/platform/httpx"
	"github.com/AlikoYe/TableBot/backend/internal/venue"
	"github.com/AlikoYe/TableBot/backend/static"
)

// Notifier is everything the HTTP side tells users about; bot.Notifier implements it.
type Notifier interface {
	booking.Notifier
	owner.Notifier
}

// services are shared by the HTTP router, the bot callbacks and the worker.
type services struct {
	venues      *venue.Service
	bookings    *booking.Service
	bookingRepo *booking.Repo
	owner       *owner.Service
}

func newServices(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, notifier Notifier) services {
	venueSvc := venue.NewService(venue.NewRepo(pool))
	bookingRepo := booking.NewRepo(pool)
	return services{
		venues:      venueSvc,
		bookings:    booking.NewService(bookingRepo, venueSvc, notifier, cfg.HoldMinutes, log),
		bookingRepo: bookingRepo,
		owner:       owner.NewService(owner.NewRepo(pool), bookingRepo, notifier, log),
	}
}

func newRouter(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, svc services) http.Handler {
	ownerSvc := svc.owner
	ownerHandler := owner.NewHandler(ownerSvc, log)
	venues := venue.NewHandler(svc.venues, log)
	bookings := booking.NewHandler(svc.bookings, log)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(httpx.RequestLogger(log))
	r.Use(httpx.Recoverer(log))
	r.NotFound(httpx.NotFound)
	r.MethodNotAllowed(httpx.MethodNotAllowed)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", healthHandler(pool))
		r.Get("/static/plans/{file}", static.PlanHandler)

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(auth.Options{
				BotToken: cfg.BotToken,
				AppEnv:   cfg.AppEnv,
				Users:    auth.NewUserRepo(pool),
				Logger:   log,
			}))

			r.Get("/me", auth.MeHandler(ownerSvc, log))
			r.Route("/venues", venues.Routes)
			r.Route("/bookings", bookings.Routes)
			r.Route("/owner", ownerHandler.Routes)
		})
	})
	return r
}

func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, httpx.CodeUnavailable, "database is unreachable")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
