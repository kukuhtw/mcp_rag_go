// internal/app/routes_test.go  (tambahan import & routes)

package app_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	apppkg "mcp-oilgas/internal/app"
)

// Pastikan /admin/* diproteksi (tanpa auth tidak boleh 200)
func TestAdminRoutesProtected(t *testing.T) {
	r := mux.NewRouter()
	apppkg.RegisterRoutes(r)

	// tanpa kredensial
	req := httptest.NewRequest(http.MethodGet, "/admin/docs", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected non-200 for protected admin route, got 200")
	}
}

// Sanity check: public endpoints tetap 200
func TestPublicRoutesHealthy(t *testing.T) {
	r := mux.NewRouter()
	apppkg.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /healthz, got %d", rec.Code)
	}
}
