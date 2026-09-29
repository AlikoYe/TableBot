package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alikhanyedilov/tablebot/backend/internal/auth"
	"github.com/alikhanyedilov/tablebot/backend/internal/booking"
	"github.com/alikhanyedilov/tablebot/backend/internal/platform/config"
	"github.com/alikhanyedilov/tablebot/backend/internal/platform/httpx"
	"github.com/alikhanyedilov/tablebot/backend/internal/restaurant"
	"github.com/alikhanyedilov/tablebot/backend/internal/staff"
)

func newRouter(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) http.Handler {
	restaurants := restaurant.NewHandler(restaurant.NewService(restaurant.NewRepo(pool)), log)
	bookings := booking.NewHandler(booking.NewService(booking.NewRepo(pool)), log)
	staffSvc := staff.NewService(staff.NewRepo(pool))
	staffHandler := staff.NewHandler(staffSvc, log)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(httpx.RequestLogger(log))
	r.Use(httpx.Recoverer(log))
	r.NotFound(httpx.NotFound)
	r.MethodNotAllowed(httpx.MethodNotAllowed)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", healthHandler(pool))

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(auth.Options{
				BotToken: cfg.BotToken,
				AppEnv:   cfg.AppEnv,
				Logger:   log,
			}))

			r.Get("/me", auth.MeHandler(staffSvc, log))
			r.Route("/restaurants", restaurants.Routes)
			r.Route("/bookings", bookings.Routes)
			r.Route("/staff", staffHandler.Routes)
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
