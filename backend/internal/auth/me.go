package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/AlikoYe/TableBot/backend/internal/platform/httpx"
)

// VenueOwnership lists the venues a user owns (venues.owner_id = telegram_id).
type VenueOwnership interface {
	OwnedVenueIDs(ctx context.Context, telegramID int64) ([]int64, error)
}

type meResponse struct {
	TelegramID    int64   `json:"telegram_id"`
	Name          string  `json:"name"`
	Username      *string `json:"username"`
	Role          string  `json:"role"`
	IsOwner       bool    `json:"is_owner"`
	OwnedVenueIDs []int64 `json:"owned_venue_ids"`
}

// MeHandler serves GET /api/me.
func MeHandler(venues VenueOwnership, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := CurrentUser(r)
		if !ok {
			unauthorized(w, "not authenticated")
			return
		}
		ids, err := venues.OwnedVenueIDs(r.Context(), user.ID)
		if err != nil {
			log.ErrorContext(r.Context(), "owned venues", slog.Any("err", err))
			httpx.Internal(w)
			return
		}
		if ids == nil {
			ids = []int64{}
		}
		resp := meResponse{
			TelegramID:    user.ID,
			Name:          user.DisplayName(),
			Role:          user.Role,
			IsOwner:       len(ids) > 0,
			OwnedVenueIDs: ids,
		}
		if user.Username != "" {
			resp.Username = &user.Username
		}
		httpx.JSON(w, http.StatusOK, resp)
	}
}
