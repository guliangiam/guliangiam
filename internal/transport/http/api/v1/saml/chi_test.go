package saml

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

func TestRealRoutes_SamlRedirect(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodGet, "/api/v1/auth/saml/redirect/org/app", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_SamlAcs(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodPost, "/api/v1/auth/saml/acs/org/app", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_SamlMetadata(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodGet, "/api/v1/auth/saml/metadata/org/app", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}
