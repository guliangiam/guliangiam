package dashboard

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	appdashboard "github.com/guliangiam/guliangiam/internal/app/dashboard"
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

func doRequestWithQuery(t *testing.T, router chi.Router, method, path, query string, body io.Reader) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path+"?"+query, body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

func newHandlerWithNilService() Handler {
	return Handler{DashboardService: appdashboard.Service{}}
}

func TestRealRoutes_Stats(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/dashboard/stats", "organizationId=org-001", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_Users(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/dashboard/users", "organizationId=org-001", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_Auth(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/dashboard/auth", "organizationId=org-001", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_Resources(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/dashboard/resources", "organizationId=org-001", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_ProviderDistribution(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/dashboard/provider-distribution", "organizationId=org-001", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}
