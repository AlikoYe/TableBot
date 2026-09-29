package staff

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/alikhanyedilov/tablebot/backend/internal/auth"
	"github.com/alikhanyedilov/tablebot/backend/internal/platform/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts under /api/staff. Every route requires a staff role.
func (h *Handler) Routes(r chi.Router) {
	r.Use(h.requireStaff)
	r.Get("/bookings", h.bookings)
	r.Post("/bookings/{id}/confirm", h.confirm)
}

func (h *Handler) requireStaff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.CurrentUser(r)
		if !ok {
			httpx.Error(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "not authenticated")
			return
		}
		role, err := h.svc.RoleFor(r.Context(), user.ID)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		if !IsStaffRole(role) {
			httpx.Error(w, http.StatusForbidden, httpx.CodeForbidden, "staff access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) bookings(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.CurrentUser(r)
	date, err := httpx.QueryDate(r, "date")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	items, err := h.svc.BookingsForDate(r.Context(), user.ID, date)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"date":  date.Format(httpx.DateLayout),
		"items": items,
	})
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.CurrentUser(r)
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	b, err := h.svc.Confirm(r.Context(), user.ID, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, b)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "staff handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}
