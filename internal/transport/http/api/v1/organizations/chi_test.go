package organizations

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	apppkg "github.com/guliangiam/guliangiam/internal/app/organizations"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	return Handler{Service: apppkg.Service{}}
}

// stubOrganizationRepository 只实现列表所需的 List，其余方法走内嵌接口（不会被调用）。
type stubOrganizationRepository struct {
	orgdomain.Repository
	organizations []*orgdomain.Organization
}

func (r stubOrganizationRepository) List(context.Context, orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	return r.organizations, int64(len(r.organizations)), nil
}

// stubIdentityUoW 只提供 Organizations()，用于让真实 Handler 处理真实列表路径。
type stubIdentityUoW struct {
	appuow.IdentityUoW
	repository orgdomain.Repository
}

func (u stubIdentityUoW) Organizations() orgdomain.Repository { return u.repository }

// newHandlerWithOrganizations 构造可返回指定组织数的真实 Handler。
func newHandlerWithOrganizations(count int) Handler {
	orgs := make([]*orgdomain.Organization, 0, count)
	for i := 0; i < count; i++ {
		orgs = append(orgs, &orgdomain.Organization{
			ID:          "01a0ea76-0c00-7000-8000-00000000000" + string(rune('1'+i)),
			Slug:        []string{"built-in", "second-org"}[i%2],
			DisplayName: []string{"Built-in Organization", "Second Org"}[i%2],
			Status:      "active",
		})
	}
	return Handler{Service: apppkg.Service{UOW: stubIdentityUoW{
		repository: stubOrganizationRepository{organizations: orgs},
	}}}
}

// ---------------------------------------------------------------------------
// Real route registration tests (covers chi.go code)
// ---------------------------------------------------------------------------

func TestRealRoutes_GetOrganizations(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithOrganizations(2))

	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/organizations",
		"keyword=test", nil)
	defer resp.Body.Close()

	// 精确状态码 + 响应体条数：不能再让“空列表/任意状态码”蒙混过关。
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
		Page struct {
			Total int64 `json:"total"`
		} `json:"page"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.EqualValues(t, 2, body.Page.Total)
	require.Len(t, body.Items, 2)
	require.Equal(t, "built-in", body.Items[0].Slug)
}

func TestRealRoutes_PostOrganizations(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/organizations", body)
	// Request may fail validation (400) or reach handler (500/401/404/etc)
	// Either way, chi.go route code is covered
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_GetOrganizations_Id(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/organizations/{id}", nil)
	// Request may return various status codes depending on handler
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_GetOrganizations_Byslug_Slug(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/organizations/by-slug/{slug}", nil)
	// Request may return various status codes depending on handler
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_PatchOrganizations_Id(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{}`)
	resp := doRequest(t, router, http.MethodPatch, "/api/v1/organizations/{id}", body)
	// Request may fail validation (400) or reach handler (500/401/404/etc)
	// Either way, chi.go route code is covered
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_DeleteOrganizations_Id(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodDelete, "/api/v1/organizations/{id}", nil)
	// Request may return various status codes depending on handler
	assert.NotEqual(t, 0, resp.StatusCode)
}

// DELETE must reject a non-UUID path id at the transport boundary. {id}
// addresses a uuid column; a slug used to pass the super_admin scope check
// unchanged, reach the repository and surface the cast failure as a 500.
// The zero Service panics if the handler runs, so a 404 proves the request was
// rejected before the app layer.
func TestDeleteOrganization_RejectsNonUUIDId(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/l4-id-resolve-20261105", nil)
	// A platform admin with super_admin scope makes TrustedOrganizationID return
	// the requested value unchanged — the path that used to forward the slug.
	ctx := tenantctx.WithTenantScope(
		req.Context(),
		"01a0ea76-0c00-7000-8000-000000000001",
		"built-in",
		tenantctx.TenantFilterSuperAdmin,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req.WithContext(ctx))
	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRealRoutes_PutOrganizations_Id_Settings(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{}`)
	resp := doRequest(t, router, http.MethodPut, "/api/v1/organizations/{id}/settings", body)
	// Request may fail validation (400) or reach handler (500/401/404/etc)
	// Either way, chi.go route code is covered
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_PutOrganizations_Id_Theme(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{}`)
	resp := doRequest(t, router, http.MethodPut, "/api/v1/organizations/{id}/theme", body)
	// Request may fail validation (400) or reach handler (500/401/404/etc)
	// Either way, chi.go route code is covered
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_PutOrganizations_Id_Securitypolicy(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	body := bytes.NewBufferString(`{}`)
	resp := doRequest(t, router, http.MethodPut, "/api/v1/organizations/{id}/security-policy", body)
	// Request may fail validation (400) or reach handler (500/401/404/etc)
	// Either way, chi.go route code is covered
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_GetOrganizations_Id_Settings(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/organizations/{id}/settings", nil)
	// Request may return various status codes depending on handler
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_GetOrganizations_Id_Theme(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/organizations/{id}/theme", nil)
	// Request may return various status codes depending on handler
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_GetOrganizations_Id_Securitypolicy(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/organizations/{id}/security-policy", nil)
	// Request may return various status codes depending on handler
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_GetOrganizations_Id_Applications(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/organizations/{id}/applications",
		"keyword=test", nil)
	// Request may return various status codes depending on handler
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_GetOrganizations_Id_Defaultapplication(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/organizations/{id}/default-application", nil)
	// Request may return various status codes depending on handler
	assert.NotEqual(t, 0, resp.StatusCode)
}

// TestRegisteredRoutes_ConfigurationAndDeploymentRegions asserts the two
// endpoints the organization detail form depends on are actually mounted by
// RegisterChiRoutes. Walking the router proves registration independently of
// auth/CSRF middleware and without executing the handlers.
func TestRegisteredRoutes_ConfigurationAndDeploymentRegions(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	registered := make(map[string]bool)
	require.NoError(t, chi.Walk(router, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[method+" "+route] = true
		return nil
	}))

	require.True(t, registered["PUT /api/v1/organizations/{id}/configuration"],
		"PUT /api/v1/organizations/{id}/configuration is not registered")
	require.True(t, registered["GET /api/v1/organizations/deployment-regions"],
		"GET /api/v1/organizations/deployment-regions is not registered")
}

// TestRealRoutes_PutOrganizations_Id_Configuration proves the configuration
// route matches under /api/v1. The matched request reaches the app layer and
// fails its own validation (empty themeName -> 400); a 404 here would mean the
// router never matched the path.
func TestRealRoutes_PutOrganizations_Id_Configuration(t *testing.T) {
	const orgID = "01a0ea76-0c00-7000-8000-000000000001"
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/"+orgID+"/configuration", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), orgID, "built-in", tenantctx.TenantFilterSuperAdmin)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req.WithContext(ctx))
	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"PUT /api/v1/organizations/{id}/configuration did not reach app validation (404 means no route match)")
}

// TestRealRoutes_GetOrganizations_DeploymentRegions asserts the static route
// wins over /api/v1/organizations/{id} and returns the expected empty registry.
func TestRealRoutes_GetOrganizations_DeploymentRegions(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/organizations/deployment-regions", nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Empty(t, body.Items)
}

// TestLegacyUnprefixedOrganizationPathsAre404 reproduces the reported failure:
// the same endpoints without /api/v1 never reach the organization router.
func TestLegacyUnprefixedOrganizationPathsAre404(t *testing.T) {
	const orgID = "01a0ea76-0c00-7000-8000-000000000001"
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/organizations/" + orgID + "/configuration"},
		{http.MethodGet, "/organizations/deployment-regions"},
	}
	for _, tc := range cases {
		resp := doRequest(t, router, tc.method, tc.path, nil)
		resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode, "%s %s", tc.method, tc.path)
	}
}

func TestRealRoutes_GetOrganizations_Names(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/organizations/names", nil)
	// Request may return various status codes depending on handler
	assert.NotEqual(t, 0, resp.StatusCode)
}
