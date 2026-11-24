package sessions

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

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

func doRequestWithQuery(t *testing.T, router chi.Router, method, path, query string, body io.Reader) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path+"?"+query, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

func newHandlerWithNilService() Handler {
	return Handler{Service: appsessions.Service{}}
}

// ---------------------------------------------------------------------------
// Real route registration tests (covers chi.go code)
// ---------------------------------------------------------------------------

func TestRealRoutes_ListSessions_ParsesQueryParams(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/sessions",
		"organizationId=org-001&userId=user-001&applicationId=app-001&state=active&activeOnly=true&keyword=test", nil)
	// No authenticated tenant scope: the request value is not trusted.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_GetSession_ExtractsPathParam(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/sessions/session-001", nil)
	// No authenticated tenant scope: the by-ID route rejects before the service.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_DeleteSession_ExtractsPathParam(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodDelete, "/api/v1/sessions/session-001", nil)
	// No authenticated tenant scope: the by-ID route rejects before the service.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_RevokeSession_ExtractsPathParam(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodPost, "/api/v1/sessions/session-001/revoke", nil)
	// No authenticated tenant scope: the by-ID route rejects before the service.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_PatchSession_ParsesBodyAndParam(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{"sessionState": "inactive", "revision": 1}`)
	resp := doRequest(t, router, http.MethodPatch, "/api/v1/sessions/session-001", body)
	// No authenticated tenant scope: the by-ID route rejects before the service.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_IsDuplicated_ParsesBody(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{"userId": "user-001", "currentSessionId": "session-001"}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/sessions/is-duplicated", body)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// Note: /api/v1/sessions/current and /api/v1/sessions/current/revoke-others
// require authtransport.OpaqueSessionIDFromRequest which returns false without
// proper auth context, so they return 401 Unauthorized (not panic).
func TestRealRoutes_CurrentSession_NoAuth_ReturnsUnauthorized(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/sessions/current", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRealRoutes_RevokeOthers_NoAuth_ReturnsUnauthorized(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodPost, "/api/v1/sessions/current/revoke-others", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
