package static

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/TableBot/backend/internal/platform/httpx"
)

// PlanCacheControl lets clients cache plans for 30 days; rename the file when a plan changes.
const PlanCacheControl = "public, max-age=2592000"

// PlanHandler serves GET /api/static/plans/{file}. No auth.
func PlanHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "file")
	if path.Ext(name) != ".svg" {
		httpx.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(Plans, "plans/"+name)
	if err != nil {
		httpx.NotFound(w, r)
		return
	}
	sum := sha256.Sum256(data)

	h := w.Header()
	h.Set("Content-Type", "image/svg+xml")
	h.Set("Cache-Control", PlanCacheControl)
	h.Set("ETag", `"`+hex.EncodeToString(sum[:8])+`"`)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
