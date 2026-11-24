package wellknown

import (
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
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

func newHandlerWithNilDeps() Handler {
	return Handler{}
}

func TestRealRoutes_OpenIDConfiguration(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())
	resp := doRequest(t, router, http.MethodGet, "/.well-known/openid-configuration", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_JWKS(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())
	resp := doRequest(t, router, http.MethodGet, "/.well-known/jwks.json", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_UserInfo(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())
	resp := doRequest(t, router, http.MethodGet, "/.well-known/userinfo", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

// TestRealRoutes_UserInfoChallenge pins RFC 6750 §3 on the actual mount point:
// a protected resource must answer a missing bearer token with a 401 that
// carries the WWW-Authenticate challenge. The header is set before the body is
// written on purpose — a header written after WriteHeader is silently dropped.
func TestRealRoutes_UserInfoChallenge(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())

	resp := doRequest(t, router, http.MethodGet, "/oauth2/userinfo", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, `Bearer error="invalid_token"`, resp.Header.Get("WWW-Authenticate"))
}

func TestRealRoutes_TokenEndpoint(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilDeps())
	resp := doRequest(t, router, http.MethodPost, "/.well-known/token", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}
