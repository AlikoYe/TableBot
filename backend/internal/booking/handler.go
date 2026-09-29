package booking

import (
	"errors"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/alikhanyedilov/tablebot/backend/internal/auth"
	"github.com/alikhanyedilov/tablebot/backend/internal/platform/httpx"
)

const (
	maxGuests         = 20
	maxSpecialRequest = 500
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts under /api/bookings.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/", h.create)
	r.Get("/my", h.my)
	r.Post("/{id}/cancel", h.cancel)
}

type createRequest struct {
	RestaurantID   int64     `json:"restaurant_id"`
	StartsAt       time.Time `json:"starts_at"`
	Guests         int       `json:"guests"`
	SpecialRequest string    `json:"special_request"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.CurrentUser(r)

	var req createRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	switch {
	case req.RestaurantID <= 0:
		httpx.BadRequest(w, "restaurant_id is required")
		return
	case req.StartsAt.IsZero():
		httpx.BadRequest(w, "starts_at is required (RFC 3339)")
		return
	case req.Guests < 1 || req.Guests > maxGuests:
		httpx.BadRequest(w, "guests must be between 1 and 20")
		return
	case utf8.RuneCountInString(req.SpecialRequest) > maxSpecialRequest:
		httpx.BadRequest(w, "special_request must be at most 500 characters")
		return
	}

	b, err := h.svc.Create(r.Context(), CreateInput{
		UserID:         user.ID,
		RestaurantID:   req.RestaurantID,
		StartsAt:       req.StartsAt,
		Guests:         req.Guests,
		SpecialRequest: req.SpecialRequest,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, b)
}

func (h *Handler) my(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.CurrentUser(r)
	items, err := h.svc.ListForUser(r.Context(), user.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.CurrentUser(r)
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	b, err := h.svc.Cancel(r.Context(), user.ID, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, b)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrNotOwner):
		// Don't reveal that someone else's booking exists.
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, ErrNotFound.Error())
	case errors.Is(err, ErrSlotFull), errors.Is(err, ErrNotActive):
		httpx.Error(w, http.StatusConflict, httpx.CodeConflict, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "booking handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}
