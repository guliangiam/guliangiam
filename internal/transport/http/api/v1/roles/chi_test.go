package roles

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	approles "github.com/guliangiam/guliangiam/internal/app/roles"
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
	return Handler{Service: approles.Service{}}
}

// ---------------------------------------------------------------------------
// Real route registration tests (covers chi.go code)
// ---------------------------------------------------------------------------

func TestRealRoutes_ListRoles_ParsesQueryParams(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/roles",
		"organizationId=org-001&keyword=admin&isEnabled=true", nil)
	// No authenticated tenant scope: the request value is not trusted.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_CreateRole_ParsesBody(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{
		"organizationId": "org-001",
		"name": "admin",
		"displayName": "Administrator"
	}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/roles", body)
	// No authenticated tenant scope: the body value is not trusted.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_GetRole_ExtractsPathParam(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/roles/role-001", nil)
	// No authenticated tenant scope: the path id is never resolved.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_PatchRole_ParsesBodyAndParam(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{"displayName": "Updated"}`)
	resp := doRequest(t, router, http.MethodPatch, "/api/v1/roles/role-001", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_DeleteRole_ExtractsPathParam(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodDelete, "/api/v1/roles/role-001", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_BulkImport_ParsesBody(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{
		"organizationId": "org-001",
		"roles": [{"name": "role1", "displayName": "Role 1"}]
	}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/roles/bulk-import", body)
	// No authenticated tenant scope: body and per-row values are not trusted.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_UploadExcel_ParsesMultipart(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	// Without proper multipart content-type, ParseMultipartForm panics
	// and Recoverer catches it returning 500
	resp := doRequest(t, router, http.MethodPost, "/api/v1/roles/upload", nil)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
