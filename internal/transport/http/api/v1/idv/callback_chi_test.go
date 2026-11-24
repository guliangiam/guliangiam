package idv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	domainidv "github.com/guliangiam/guliangiam/internal/domain/idv"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
)

func newCallbackRouter(t *testing.T) chi.Router {
	t.Helper()
	router := chi.NewRouter()
	RegisterCallbackChiRoutes(router, Handler{})
	return router
}

func TestCallbackRouteRejectsOversizedBody(t *testing.T) {
	router := newCallbackRouter(t)
	body := strings.Repeat("a", maxCallbackBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/idv/callback/jumio", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestCallbackRouteRejectsEmptyBody(t *testing.T) {
	router := newCallbackRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/idv/callback/jumio", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCallbackRoutePassesBodyToService(t *testing.T) {
	// A zero-value service has no provider wiring, so the handler must surface
	// 503 unavailable — proving the body was accepted (not 400/413) and handed
	// to the service path rather than being parsed or dropped in transport.
	router := newCallbackRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/idv/callback/jumio", strings.NewReader(`{"jumioIdScanReference":"scan-1"}`))
	req.Header.Set("X-Jumio-Signature", "deadbeef")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// TestCallbackRouteIsRateLimited proves the public callback is covered by the
// same platform rate-limit middleware the root router mounts, and that the
// happy path stays 200: a wired Service accepts the signed callback, then a
// second request from the same client IP is denied with 429 (not 500) before
// the handler runs. Rate 0.5 req/s makes the refill exactly 2 seconds, so the
// Retry-After value also documents its unit.
func TestCallbackRouteIsRateLimited(t *testing.T) {
	repo := newCallbackRepo()
	if err := repo.Create(context.Background(), pendingRecord("ver-rl", "jumio", "scan-rl")); err != nil {
		t.Fatal(err)
	}

	router := chi.NewRouter()
	router.Use(mw.RateLimit(mw.RateLimiterConfig{
		Rate:            0.5,
		Burst:           1,
		CleanupInterval: time.Minute,
		MaxAge:          time.Minute,
		PolicyID:        "idv-callback-test",
	}))
	RegisterCallbackChiRoutes(router, Handler{Service: newCallbackService(repo, domainidv.ProviderCredentials{
		Provider: "jumio", APIKey: "token", APISecret: "secret", WebhookSecret: "whsec",
	})})

	payload := []byte(`{"jumioIdScanReference":"scan-rl","status":"APPROVED_VERIFIED"}`)
	post := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/idv/callback/jumio", strings.NewReader(string(payload)))
		req.Header.Set("X-Jumio-Signature", signed("whsec", payload))
		return req
	}

	first := httptest.NewRecorder()
	router.ServeHTTP(first, post())
	t.Logf("first request -> %d", first.Code)
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", first.Code)
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, post())
	t.Logf("second request -> %d (Retry-After=%q seconds)", second.Code, second.Header().Get("Retry-After"))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", second.Code)
	}
	if got := second.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("Retry-After = %q, want \"2\" (delta-seconds)", got)
	}
}
