package static

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newServer() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/static/plans/{file}", PlanHandler)
	return r
}

func get(t *testing.T, url string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	newServer().ServeHTTP(rec, req)
	return rec
}

func TestPlanHandlerServesSVG(t *testing.T) {
	rec := get(t, "/api/static/plans/nomad-bistro.svg", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != PlanCacheControl {
		t.Errorf("Cache-Control = %q", cc)
	}
	if !strings.Contains(rec.Body.String(), `viewBox="0 0 1000 700"`) {
		t.Error("plan must use viewBox 0 0 1000 700")
	}

	etag := rec.Header().Get("ETag")
	if rec := get(t, "/api/static/plans/nomad-bistro.svg", map[string]string{"If-None-Match": etag}); rec.Code != http.StatusNotModified {
		t.Errorf("conditional GET status = %d, want 304", rec.Code)
	}
}

func TestPlanHandlerNotFound(t *testing.T) {
	for _, url := range []string{
		"/api/static/plans/nope.svg",
		"/api/static/plans/embed.go",
		"/api/static/plans/..%2Fembed.go",
	} {
		rec := get(t, url, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", url, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"error":"not_found"`) {
			t.Errorf("%s: body = %s", url, rec.Body)
		}
	}
}

func TestAllPlansUseSameViewBox(t *testing.T) {
	files, err := fs.Glob(Plans, "plans/*.svg")
	if err != nil || len(files) == 0 {
		t.Fatalf("no embedded plans: %v", err)
	}
	for _, f := range files {
		data, _ := fs.ReadFile(Plans, f)
		if !strings.Contains(string(data), `viewBox="0 0 1000 700"`) {
			t.Errorf("%s: viewBox must be 0 0 1000 700", f)
		}
	}
}

func TestSeedPlanURLsExist(t *testing.T) {
	seed, err := os.ReadFile("../seed/dev.sql")
	if err != nil {
		t.Fatal(err)
	}
	urls := regexp.MustCompile(`/api/static/plans/[\w.-]+`).FindAllString(string(seed), -1)
	if len(urls) == 0 {
		t.Fatal("seed has no floor_plan_url values")
	}
	for _, u := range urls {
		if rec := get(t, u, nil); rec.Code != http.StatusOK {
			t.Errorf("seed URL %s: status %d", u, rec.Code)
		}
	}
}
