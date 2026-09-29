package restaurant

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/alikhanyedilov/tablebot/backend/internal/platform/httpx"
)

// MaxGuests bounds the party size accepted by the API.
const MaxGuests = 20

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts under /api/restaurants.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Get("/{id}/slots", h.slots)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	rest, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rest)
}

func (h *Handler) slots(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	date, err := httpx.QueryDate(r, "date")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	guests, err := httpx.QueryInt(r, "guests", 1, MaxGuests)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}

	slots, err := h.svc.Slots(r.Context(), id, date, guests)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"date":   date.Format(httpx.DateLayout),
		"guests": guests,
		"items":  slots,
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "restaurant handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}
