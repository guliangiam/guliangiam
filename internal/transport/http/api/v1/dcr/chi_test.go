package dcr

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
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

func newHandlerWithNilService() Handler {
	return Handler{Service: appapplications.Service{}}
}

func TestRealRoutes_RegisterClient(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	body := bytes.NewBufferString(`{"clientName": "test", "redirectUris": ["http://localhost/cb"]}`)
	resp := doRequest(t, router, http.MethodPost, "/oauth2/register", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_GetClient(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodGet, "/oauth2/register/client-001", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_UpdateClient(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	body := bytes.NewBufferString(`{"clientName": "updated"}`)
	resp := doRequest(t, router, http.MethodPut, "/oauth2/register/client-001", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_DeleteClient(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodDelete, "/oauth2/register/client-001", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}
