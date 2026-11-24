package radius

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

func testRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	return r
}

func doRequest(t *testing.T, router chi.Router, method, path string, body io.Reader) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

func newHandlerWithNilDeps() *Handler {
	return &Handler{}
}

func TestRealRoutes_StartServer(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())
	body := bytes.NewBufferString(`{"addr": ":1812", "secret": "testsecret"}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/radius/start", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_StopServer(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())
	resp := doRequest(t, router, http.MethodPost, "/api/v1/radius/stop", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_GetStatus(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())
	resp := doRequest(t, router, http.MethodGet, "/api/v1/radius/status", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}
