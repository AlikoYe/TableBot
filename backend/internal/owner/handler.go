package owner

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/TableBot/backend/internal/platform/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts under /api/owner. Each handler must call CheckVenue or CheckBooking.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/bookings", httpx.NotImplementedHandler)
	r.Post("/bookings/{id}/confirm", httpx.NotImplementedHandler)
	r.Post("/bookings/{id}/reject", httpx.NotImplementedHandler)
}
