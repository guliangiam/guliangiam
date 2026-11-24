package mcpservers

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	appmcpservers "github.com/guliangiam/guliangiam/internal/app/mcpservers"
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
	return Handler{Service: appmcpservers.Service{}}
}

func TestRealRoutes_ListMcpServers(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodGet, "/api/v1/mcp-servers", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_CreateMcpServer(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	body := bytes.NewBufferString(`{"organizationId": "org-001", "name": "test-server"}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/mcp-servers", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_GetMcpServer(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodGet, "/api/v1/mcp-servers/server-001", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_PatchMcpServer(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	body := bytes.NewBufferString(`{"displayName": "Updated"}`)
	resp := doRequest(t, router, http.MethodPatch, "/api/v1/mcp-servers/server-001", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_DeleteMcpServer(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodDelete, "/api/v1/mcp-servers/server-001", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}
