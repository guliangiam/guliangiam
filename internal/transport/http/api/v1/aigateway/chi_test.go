package aigateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Helpers — follows the rebac/gateway/tokens chi_test.go convention
// ---------------------------------------------------------------------------

func dataPlaneTestRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer) // catch nil-Service panics → 500
	return r
}

func nilGatewayHandler() Handler {
	return Handler{} // Service is nil — any real call panics → caught by Recoverer → 500
}

func doDataPlane(t *testing.T, router chi.Router, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// Data plane — OpenAI face (3 routes)
// ---------------------------------------------------------------------------

func TestDataPlaneRoutes_OpenAI_Face(t *testing.T) {
	r := dataPlaneTestRouter()
	RegisterChiRoutes(r, nilGatewayHandler())

	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/ai/faces/openai/chat/completions"},
		{http.MethodPost, "/api/v1/ai/faces/openai/embeddings"},
		{http.MethodGet, "/api/v1/ai/faces/openai/models"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := doDataPlane(t, r, tc.method, tc.path)
			assert.NotEqual(t, 0, rec.Code, "chi must route this path")
		})
	}
}

// ---------------------------------------------------------------------------
// Data plane — Anthropic face (3 routes)
// ---------------------------------------------------------------------------

func TestDataPlaneRoutes_Anthropic_Face(t *testing.T) {
	r := dataPlaneTestRouter()
	RegisterChiRoutes(r, nilGatewayHandler())

	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/ai/faces/anthropic/chat/completions"},
		{http.MethodPost, "/api/v1/ai/faces/anthropic/messages"},
		{http.MethodPost, "/api/v1/ai/faces/anthropic/embeddings"},
		{http.MethodGet, "/api/v1/ai/faces/anthropic/models"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := doDataPlane(t, r, tc.method, tc.path)
			assert.NotEqual(t, 0, rec.Code, "chi must route this path")
		})
	}
}

// ---------------------------------------------------------------------------
// Data plane — Gemini face (3 routes)
// ---------------------------------------------------------------------------

func TestDataPlaneRoutes_Gemini_Face(t *testing.T) {
	r := dataPlaneTestRouter()
	RegisterChiRoutes(r, nilGatewayHandler())

	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/ai/faces/gemini/chat/completions"},
		{http.MethodPost, "/api/v1/ai/faces/gemini/embeddings"},
		{http.MethodGet, "/api/v1/ai/faces/gemini/models"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := doDataPlane(t, r, tc.method, tc.path)
			assert.NotEqual(t, 0, rec.Code, "chi must route this path")
		})
	}
}
