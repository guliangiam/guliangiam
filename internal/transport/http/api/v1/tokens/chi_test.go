package tokens

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	apptokens "github.com/guliangiam/guliangiam/internal/app/tokens"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
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

// newHandlerWithNilService creates a Handler with a zero-value Service.
// The Service methods will panic because UOW is nil, but the chi.go route
// registration code (request parsing, parameter extraction) will be covered.
// The Recoverer middleware catches panics and returns 500.
func newHandlerWithNilService() Handler {
	return Handler{Service: apptokens.Service{}}
}

// ---------------------------------------------------------------------------
// Real route registration tests (covers chi.go code)
// These tests call RegisterChiRoutes with a nil-UOW Handler.
// The chi.go route registration code (request parsing) is covered.
// Handler method calls panic (nil UOW), Recoverer returns 500.
// ---------------------------------------------------------------------------

func TestRealRoutes_ListTokens_ParsesQueryParams(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/tokens",
		"organizationId=org-001&userId=user-001&applicationId=app-001&activeOnly=true&keyword=test", nil)
	// No authenticated tenant scope: the query value is not trusted (400).
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_CreateToken_ParsesBody(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{
		"organizationId": "org-001",
		"userId": "user-001",
		"applicationId": "app-001",
		"scopes": ["read"],
		"expiresInSeconds": 3600
	}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/tokens", body)
	// No authenticated tenant scope: the body value is not trusted (400).
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_RevokeMany_ParsesBody(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{"organizationId": "org-001", "userId": "user-001"}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/tokens/revoke", body)
	// No authenticated tenant scope: the body value is not trusted (400).
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_Introspect_ParsesBody(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{"token": "abc123"}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/tokens/introspect", body)
	// No authenticated tenant scope (400) before the service is reached.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_Refresh_ParsesBody(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{"refreshToken": "rt-abc123"}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/tokens/refresh", body)
	// No authenticated tenant scope (400) before the service is reached.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_GetToken_ExtractsPathParam(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/tokens/token-001", nil)
	// No authenticated tenant scope (400) before the service is reached.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_RevokeToken_ExtractsPathParam(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodPost, "/api/v1/tokens/token-001/revoke", nil)
	// No authenticated tenant scope (400) before the service is reached.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_DeleteToken_ExtractsPathParam(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodDelete, "/api/v1/tokens/token-001", nil)
	// No authenticated tenant scope (400) before the service is reached.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// BE-D-19: transport 响应 DTO 只投影 tokenHashSet/tokenHashMasked，
// 不得把应用层已脱敏的读模型退化回原始 tokenHash 字段。
func TestNewListTokensResponse_MasksTokenHash(t *testing.T) {
	response := newListTokensResponse([]apptokens.TokenListItem{
		{ID: "tok-1", TokenHashSet: true, TokenHashMasked: "******"},
		{ID: "tok-2"},
	}, presenter.PageMeta{Page: 1, PageSize: 10, Total: 2})

	body, err := json.Marshal(response)
	assert.NoError(t, err)
	assert.NotContains(t, string(body), `"tokenHash"`)
	assert.Contains(t, string(body), `"tokenHashSet":true`)
	assert.Contains(t, string(body), `"tokenHashMasked":"******"`)
	assert.Contains(t, string(body), `"tokenHashSet":false`)
	assert.True(t, response.Items[0].TokenHashSet)
	assert.Equal(t, "******", response.Items[0].TokenHashMasked)
	assert.False(t, response.Items[1].TokenHashSet)
	assert.Empty(t, response.Items[1].TokenHashMasked)
}
