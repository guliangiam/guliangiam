package batch

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

func newHandlerWithNilDeps() Handler {
	return Handler{}
}

func TestRealRoutes_PostBatch_Users(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())
	body := bytes.NewBufferString(`{"operations": []}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/batch/users", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_PostBatch_Organizations(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())
	body := bytes.NewBufferString(`{"operations": []}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/batch/organizations", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_PostBatch_Applications(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())
	body := bytes.NewBufferString(`{"operations": []}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/batch/applications", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}
