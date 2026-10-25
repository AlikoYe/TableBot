package owner

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/TableBot/backend/internal/auth"
	"github.com/AlikoYe/TableBot/backend/internal/platform/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts under /api/owner. Access is checked per venue inside the service.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/bookings", h.list)
	r.Post("/bookings/{id}/confirm", h.decide(true))
	r.Post("/bookings/{id}/reject", h.decide(false))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.CurrentUser(r)
	venueID, err := httpx.QueryID(r, "venue_id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	date, err := httpx.QueryDate(r, "date")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	items, err := h.svc.List(r.Context(), user.ID, venueID, date)
	if err != nil {
		h.fail(w, r, err, "venue not found")
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) decide(confirm bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := auth.CurrentUser(r)
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.BadRequest(w, err.Error())
			return
		}
		b, err := h.svc.Decide(r.Context(), user.ID, id, confirm)
		if err != nil {
			h.fail(w, r, err, "booking not found")
			return
		}
		httpx.JSON(w, http.StatusOK, b)
	}
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error, notFound string) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, notFound)
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, http.StatusForbidden, httpx.CodeForbidden, err.Error())
	case errors.Is(err, ErrNotPending):
		httpx.Error(w, http.StatusConflict, httpx.CodeNotPending, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "owner handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}
