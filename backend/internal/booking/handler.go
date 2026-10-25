// Package booking handles customer bookings of a specific table.
package booking

import (
	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/TableBot/backend/internal/platform/httpx"
)

// Routes mounts under /api/bookings.
func Routes(r chi.Router) {
	r.Post("/", httpx.NotImplementedHandler)
	r.Get("/my", httpx.NotImplementedHandler)
	r.Post("/{id}/cancel", httpx.NotImplementedHandler)
}
