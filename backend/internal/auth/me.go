package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/alikhanyedilov/tablebot/backend/internal/platform/httpx"
)

// RoleResolver looks up the application role for a Telegram user.
type RoleResolver interface {
	RoleFor(ctx context.Context, telegramID int64) (string, error)
}

type meResponse struct {
	User User   `json:"user"`
	Role string `json:"role"`
}

// MeHandler serves GET /api/me: the authenticated user and their role.
func MeHandler(roles RoleResolver, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := CurrentUser(r)
		if !ok {
			unauthorized(w, "not authenticated")
			return
		}
		role, err := roles.RoleFor(r.Context(), user.ID)
		if err != nil {
			log.ErrorContext(r.Context(), "resolve role", slog.Any("err", err))
			httpx.Internal(w)
			return
		}
		httpx.JSON(w, http.StatusOK, meResponse{User: user, Role: role})
	}
}
