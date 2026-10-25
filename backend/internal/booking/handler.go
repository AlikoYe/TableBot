package booking

import (
	"errors"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/TableBot/backend/internal/auth"
	"github.com/AlikoYe/TableBot/backend/internal/platform/httpx"
	"github.com/AlikoYe/TableBot/backend/internal/venue"
)

const (
	maxGuests         = 20
	maxSpecialRequest = 300
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
	TableID        int64     `json:"table_id"`
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
	case req.TableID <= 0:
		httpx.BadRequest(w, "table_id is required")
		return
	case req.StartsAt.IsZero():
		httpx.BadRequest(w, "starts_at is required (RFC 3339)")
		return
	case req.Guests < 1 || req.Guests > maxGuests:
		httpx.BadRequest(w, "guests must be between 1 and 20")
		return
	case utf8.RuneCountInString(req.SpecialRequest) > maxSpecialRequest:
		httpx.BadRequest(w, "special_request must be at most 300 characters")
		return
	}

	b, err := h.svc.Create(r.Context(), CreateInput{
		UserID:         user.ID,
		TableID:        req.TableID,
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
	httpx.JSON(w, http.StatusOK, items)
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
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrTableNotFound):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	case errors.Is(err, ErrTooManyGuests):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeTooManyGuests, err.Error())
	case errors.Is(err, venue.ErrOutsideHours), errors.Is(err, venue.ErrTooFarAhead):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeOutsideHours, err.Error())
	case errors.Is(err, venue.ErrInPast):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeInPast, err.Error())
	case errors.Is(err, ErrTableTaken):
		httpx.Error(w, http.StatusConflict, httpx.CodeTableTaken, err.Error())
	case errors.Is(err, ErrUserBusy):
		httpx.Error(w, http.StatusConflict, httpx.CodeUserBusy, err.Error())
	case errors.Is(err, ErrNotCancellable):
		httpx.Error(w, http.StatusConflict, httpx.CodeNotCancellable, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "booking handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}
