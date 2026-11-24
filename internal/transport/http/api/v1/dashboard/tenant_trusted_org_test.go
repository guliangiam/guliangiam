package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	appdashboard "github.com/guliangiam/guliangiam/internal/app/dashboard"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	trustedDashboardOrgID = "11111111-1111-1111-1111-111111111111"
	forgedDashboardOrgID  = "22222222-2222-2222-2222-222222222222"
)

// recordingStatsRepo implements appdashboard.StatsRepository and records the
// organization argument every tenant-scoped query receives.
type recordingStatsRepo struct {
	organizations []string
}

func (r *recordingStatsRepo) record(organizationID string) {
	r.organizations = append(r.organizations, organizationID)
}

func (r *recordingStatsRepo) CountUsers(_ context.Context, organizationID string) (*appdashboard.UserCountResult, error) {
	r.record(organizationID)
	total := int64(7)
	if organizationID == allTenantsScope {
		total = 100
	}
	return &appdashboard.UserCountResult{
		Total: total, Active: total - 2, Disabled: 2,
		NewToday: 1, New7Days: 1, New30Days: 1,
	}, nil
}

func (r *recordingStatsRepo) CountOrganizations(context.Context) (int64, error) {
	return 3, nil
}

func (r *recordingStatsRepo) CountApplications(_ context.Context, organizationID string) (int64, error) {
	r.record(organizationID)
	return 1, nil
}

func (r *recordingStatsRepo) CountRoles(_ context.Context, organizationID string) (int64, error) {
	r.record(organizationID)
	return 1, nil
}

func (r *recordingStatsRepo) CountGroups(_ context.Context, organizationID string) (int64, error) {
	r.record(organizationID)
	return 1, nil
}

func (r *recordingStatsRepo) CountPermissions(_ context.Context, organizationID string) (int64, error) {
	r.record(organizationID)
	return 1, nil
}

func (r *recordingStatsRepo) CountProviders(_ context.Context, organizationID string) (int64, error) {
	r.record(organizationID)
	return 1, nil
}

func (r *recordingStatsRepo) CountActiveSessions(_ context.Context, organizationID string) (*appdashboard.SessionCountResult, error) {
	r.record(organizationID)
	return &appdashboard.SessionCountResult{Total: 4, Active: 2}, nil
}

func (r *recordingStatsRepo) CountTokens(_ context.Context, organizationID string) (*appdashboard.TokenCountResult, error) {
	r.record(organizationID)
	return &appdashboard.TokenCountResult{Total: 4, Active: 2}, nil
}

func (r *recordingStatsRepo) CountFailedLogins24h(_ context.Context, organizationID string) (int64, error) {
	r.record(organizationID)
	return 1, nil
}

func (r *recordingStatsRepo) GetLoginHeatmap(_ context.Context, organizationID string) ([]appdashboard.HeatmapPoint, error) {
	r.record(organizationID)
	return []appdashboard.HeatmapPoint{{Hour: 0, Count: 1}}, nil
}

func (r *recordingStatsRepo) GetMfaCoverageStats(_ context.Context, organizationID string) (*appdashboard.MfaCoverageResult, error) {
	r.record(organizationID)
	return &appdashboard.MfaCoverageResult{Total: 4, WithMfa: 2}, nil
}

func (r *recordingStatsRepo) GetProviderDistribution(_ context.Context, organizationID string) ([]appdashboard.ProviderDistPoint, error) {
	r.record(organizationID)
	return []appdashboard.ProviderDistPoint{{Category: "social", Count: 1}}, nil
}

func (r *recordingStatsRepo) GetTrends(_ context.Context, organizationID string) ([]appdashboard.TrendPoint, error) {
	r.record(organizationID)
	return []appdashboard.TrendPoint{{Date: "2026-11-03", Users: 1}}, nil
}

func (r *recordingStatsRepo) GetActivityHeatmap(_ context.Context, organizationID string) ([]appdashboard.ActivityPoint, error) {
	r.record(organizationID)
	return []appdashboard.ActivityPoint{{Date: "2026-11-03", Count: 1}}, nil
}

func (r *recordingStatsRepo) GetOrgUserDistribution(context.Context) ([]appdashboard.OrgUserDistPoint, error) {
	return []appdashboard.OrgUserDistPoint{{OrganizationName: "built-in", UserCount: 1}}, nil
}

type dashboardReadRoute struct {
	name string
	path string
}

// dashboardReadRoutes are the nine reads that used to pass the raw
// organizationId query parameter straight into the repository.
func dashboardReadRoutes() []dashboardReadRoute {
	return []dashboardReadRoute{
		{name: "stats", path: "/api/v1/dashboard/stats"},
		{name: "users", path: "/api/v1/dashboard/users"},
		{name: "auth", path: "/api/v1/dashboard/auth"},
		{name: "resources", path: "/api/v1/dashboard/resources"},
		{name: "provider-distribution", path: "/api/v1/dashboard/provider-distribution"},
		{name: "mfa-coverage", path: "/api/v1/dashboard/mfa-coverage"},
		{name: "login-heatmap", path: "/api/v1/dashboard/login-heatmap"},
		{name: "trends", path: "/api/v1/dashboard/trends"},
		{name: "activity-heatmap", path: "/api/v1/dashboard/activity-heatmap"},
	}
}

func dashboardRouter(repo appdashboard.StatsRepository) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{DashboardService: appdashboard.NewService(repo)})
	return router
}

// dashboardRequest attaches a tenant scope the way TenantFilterMiddleware does
// after authentication. An empty mode leaves the request without any scope.
func dashboardRequest(t *testing.T, target string, mode tenantctx.TenantFilterMode, organizationID string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if mode == "" {
		return req
	}

	ctx := tenantctx.WithTenantScope(req.Context(), organizationID, "scope-org", mode)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID:         "33333333-3333-3333-3333-333333333333",
		OrganizationID: organizationID,
	})
	return req.WithContext(ctx)
}

func assertAllQueriesUsedOrganization(t *testing.T, repo *recordingStatsRepo, want string) {
	t.Helper()
	require.NotEmpty(t, repo.organizations)
	for _, got := range repo.organizations {
		assert.Equal(t, want, got)
	}
}

// TestDashboardReads_ForgedQueryOrganizationIDIgnored pins that a regular
// caller cannot move a dashboard read to another tenant through the query
// string: every repository query receives the authenticated tenant.
func TestDashboardReads_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	for _, route := range dashboardReadRoutes() {
		t.Run(route.name, func(t *testing.T) {
			repo := &recordingStatsRepo{}
			recorder := httptest.NewRecorder()

			dashboardRouter(repo).ServeHTTP(recorder, dashboardRequest(t,
				route.path+"?organizationId="+forgedDashboardOrgID,
				tenantctx.TenantFilterStrict, trustedDashboardOrgID))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assertAllQueriesUsedOrganization(t, repo, trustedDashboardOrgID)
		})
	}
}

// TestDashboardReads_WithoutTenantScopeRejected pins the fail-fast guard: with
// no authenticated tenant, an empty organization must be a 400 rather than an
// unfiltered cross-tenant query.
func TestDashboardReads_WithoutTenantScopeRejected(t *testing.T) {
	for _, route := range dashboardReadRoutes() {
		t.Run(route.name, func(t *testing.T) {
			router := chi.NewRouter()
			RegisterChiRoutes(router, Handler{})
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, dashboardRequest(t,
				route.path+"?organizationId="+forgedDashboardOrgID, "", ""))

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
			assert.Equal(t, "invalid_argument", envelope.Error.Code)
		})
	}
}

// TestDashboardReads_PlatformAdminEmptyOrganizationKeepsAllTenants pins the
// existing product semantics: a platform administrator who does not name a
// tenant still gets the cross-tenant overview, and the trusted expansion (not
// the request) is what produces it.
func TestDashboardReads_PlatformAdminEmptyOrganizationKeepsAllTenants(t *testing.T) {
	modes := []tenantctx.TenantFilterMode{
		tenantctx.TenantFilterSuperAdmin,
		tenantctx.TenantFilterSystem,
	}
	for _, mode := range modes {
		for _, route := range dashboardReadRoutes() {
			t.Run(string(mode)+"/"+route.name, func(t *testing.T) {
				repo := &recordingStatsRepo{}
				recorder := httptest.NewRecorder()

				// The scope carries the admin's home tenant; the absence of a
				// request tenant must still mean all tenants, not the home one.
				dashboardRouter(repo).ServeHTTP(recorder, dashboardRequest(t,
					route.path, mode, trustedDashboardOrgID))

				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				assertAllQueriesUsedOrganization(t, repo, allTenantsScope)

				if route.name == "users" || route.name == "stats" {
					var body struct {
						TotalUsers int64 `json:"totalUsers"`
					}
					require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
					assert.Equal(t, int64(100), body.TotalUsers)
				}
			})
		}
	}
}

// TestDashboardReads_PlatformAdminExplicitOrganizationNarrowsScope pins that
// the query parameter remains a narrowing filter for platform administrators.
func TestDashboardReads_PlatformAdminExplicitOrganizationNarrowsScope(t *testing.T) {
	for _, route := range dashboardReadRoutes() {
		t.Run(route.name, func(t *testing.T) {
			repo := &recordingStatsRepo{}
			recorder := httptest.NewRecorder()

			dashboardRouter(repo).ServeHTTP(recorder, dashboardRequest(t,
				route.path+"?organizationId="+forgedDashboardOrgID,
				tenantctx.TenantFilterSuperAdmin, trustedDashboardOrgID))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assertAllQueriesUsedOrganization(t, repo, forgedDashboardOrgID)

			if route.name == "users" || route.name == "stats" {
				var body struct {
					TotalUsers int64 `json:"totalUsers"`
				}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
				assert.Equal(t, int64(7), body.TotalUsers)
			}
		})
	}
}
