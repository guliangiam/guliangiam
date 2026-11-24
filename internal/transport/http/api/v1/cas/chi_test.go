package cas

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
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
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

func newHandlerWithNilService() *Handler {
	return &Handler{AuthService: &appauth.Service{}}
}

func TestRealRoutes_CasValidate(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodGet, "/cas/org/app/validate", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_CasServiceValidate(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodGet, "/cas/org/app/serviceValidate", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_CasProxy(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodGet, "/cas/org/app/proxy", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_SamlValidate(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodPost, "/cas/org/app/samlValidate", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}
