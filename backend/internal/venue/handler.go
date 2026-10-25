// Package venue serves venue listings, free start times and table availability.
package venue

import (
	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/TableBot/backend/internal/platform/httpx"
)

// Routes mounts under /api/venues.
func Routes(r chi.Router) {
	r.Get("/", httpx.NotImplementedHandler)
	r.Get("/{id}", httpx.NotImplementedHandler)
	r.Get("/{id}/times", httpx.NotImplementedHandler)
	r.Get("/{id}/tables", httpx.NotImplementedHandler)
}
