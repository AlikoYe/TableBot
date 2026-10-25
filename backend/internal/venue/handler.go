package venue

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/TableBot/backend/internal/platform/httpx"
)

const maxGuests = 20

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts under /api/venues.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Get("/{id}/times", h.times)
	r.Get("/{id}/tables", h.tables)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	if category != "" && !slices.Contains(Categories, category) {
		httpx.BadRequest(w, "category must be one of "+strings.Join(Categories, ", "))
		return
	}
	items, err := h.svc.List(r.Context(), category)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	v, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *Handler) times(w http.ResponseWriter, r *http.Request) {
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
	guests, err := httpx.QueryIntDefault(r, "guests", 1, 1, maxGuests)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	items, err := h.svc.Times(r.Context(), id, date, guests)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) tables(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	// An unencoded "+" in the offset arrives as a space.
	raw := strings.ReplaceAll(r.URL.Query().Get("starts_at"), " ", "+")
	startsAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		httpx.BadRequest(w, `query parameter "starts_at" is required (RFC 3339, e.g. 2026-10-27T19:00:00+05:00)`)
		return
	}
	guests, err := httpx.QueryInt(r, "guests", 1, maxGuests)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	m, err := h.svc.Tables(r.Context(), id, startsAt, guests)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, m)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	case errors.Is(err, ErrOutsideHours), errors.Is(err, ErrTooFarAhead):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeOutsideHours, err.Error())
	case errors.Is(err, ErrInPast):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeInPast, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "venue handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}
