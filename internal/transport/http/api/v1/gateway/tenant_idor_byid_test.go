package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	appgateway "github.com/guliangiam/guliangiam/internal/app/gateway"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// tenantByIDRepo is the in-memory repository for the by-id IDOR regression
// tests. It exposes the system-scoped lookups (the pre-fix path) and the
// tenant-scoped lookups (the post-fix path) side by side so the same test
// runs before and after the fix: pre-fix the system lookup returns another
// tenant's row (200/leak), post-fix the scoped lookup returns NotFound (404).
type tenantByIDRepo struct {
	sites   map[string]*domain.Site
	servers map[string]*domain.Server

	siteUpdates   int
	siteDeletes   int
	serverUpdates int
	serverDeletes int
	systemLookups int
}

func (r *tenantByIDRepo) GetServerByID(_ context.Context, id string) (*domain.Server, error) {
	r.systemLookups++
	server, ok := r.servers[id]
	if !ok {
		return nil, apperrors.NewNotFound("server not found")
	}
	return server, nil
}

func (r *tenantByIDRepo) GetServerByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Server, error) {
	server, ok := r.servers[id]
	if !ok || server.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("server not found")
	}
	return server, nil
}

func (r *tenantByIDRepo) ListServers(context.Context, domain.ListServersQuery) ([]*domain.Server, int64, error) {
	return nil, 0, nil
}

func (r *tenantByIDRepo) CreateServer(context.Context, *domain.Server) error { return nil }

func (r *tenantByIDRepo) UpdateServer(context.Context, *domain.Server) error {
	r.serverUpdates++
	return nil
}

func (r *tenantByIDRepo) DeleteServer(context.Context, string) error {
	r.serverDeletes++
	return nil
}

func (r *tenantByIDRepo) GetSiteByID(_ context.Context, id string) (*domain.Site, error) {
	r.systemLookups++
	site, ok := r.sites[id]
	if !ok {
		return nil, apperrors.NewNotFound("site not found")
	}
	return site, nil
}

func (r *tenantByIDRepo) GetSiteByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Site, error) {
	site, ok := r.sites[id]
	if !ok || site.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("site not found")
	}
	return site, nil
}

func (r *tenantByIDRepo) ListSites(context.Context, domain.ListSitesQuery) ([]*domain.Site, int64, error) {
	return nil, 0, nil
}

func (r *tenantByIDRepo) CreateSite(context.Context, *domain.Site) error { return nil }

func (r *tenantByIDRepo) UpdateSite(context.Context, *domain.Site) error {
	r.siteUpdates++
	return nil
}

func (r *tenantByIDRepo) DeleteSite(context.Context, string) error {
	r.siteDeletes++
	return nil
}

func tenantByIDRouter(ctrl *gomock.Controller, repo domain.Repository) chi.Router {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	uow.EXPECT().Gateway().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appgateway.Service{
		Clock: forgedOrgClock{}, UUID: forgedOrgUUID{}, UOW: uow,
	}})
	return router
}

type tenantByIDAction struct {
	name   string
	method string
	path   string
	body   string
}

func tenantByIDActions() []tenantByIDAction {
	const (
		sitePath   = "/api/v1/gateway/sites/foreign-site"
		serverPath = "/api/v1/gateway/servers/foreign-server"
	)
	return []tenantByIDAction{
		{name: "get-site", method: http.MethodGet, path: sitePath},
		{name: "patch-site", method: http.MethodPatch, path: sitePath,
			body: `{"name":"hijacked","host":"evil.example.com","revision":1}`},
		{name: "delete-site", method: http.MethodDelete, path: sitePath},
		{name: "get-site-cert-status", method: http.MethodGet, path: sitePath + "/cert-status"},
		{name: "get-site-health", method: http.MethodGet, path: sitePath + "/health"},
		{name: "get-server", method: http.MethodGet, path: serverPath},
		{name: "patch-server", method: http.MethodPatch, path: serverPath,
			body: `{"name":"hijacked","endpoint":"https://evil.example.com","protocol":"http","status":"active","revision":1}`},
		{name: "delete-server", method: http.MethodDelete, path: serverPath},
	}
}

func tenantByIDFixture() *tenantByIDRepo {
	return &tenantByIDRepo{
		sites: map[string]*domain.Site{
			"foreign-site": {
				ID: "foreign-site", OrganizationID: forgedOrgRequestID,
				Name: "foreign-portal", Host: "foreign.example.com", Status: "active", Revision: 1,
			},
		},
		servers: map[string]*domain.Server{
			"foreign-server": {
				ID: "foreign-server", OrganizationID: forgedOrgRequestID,
				Name: "foreign-gateway", Endpoint: "https://foreign.example.com", Protocol: "https", Status: "active", Revision: 1,
			},
		},
	}
}

func doTenantByID(t *testing.T, router chi.Router, action tenantByIDAction, query string, withTenant bool) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if action.body != "" {
		body = strings.NewReader(action.body)
	}
	target := action.path
	if query != "" {
		target += "?" + query
	}

	var req *http.Request
	if withTenant {
		if body != nil {
			req = forgedOrgRequest(action.method, target, body)
		} else {
			req = forgedOrgRequest(action.method, target, nil)
		}
	} else {
		if body != nil {
			req = httptest.NewRequest(action.method, target, body)
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(action.method, target, nil)
		}
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestByIDRoutes_ForeignTenantHidden is the red-then-green IDOR regression:
// tenant A must not read, patch, delete, or inspect sub-resources of tenant B's
// site/server by guessing the path id. A foreign id and a missing id must both
// answer 404 and leave the row untouched.
func TestByIDRoutes_ForeignTenantHidden(t *testing.T) {
	for _, action := range tenantByIDActions() {
		action := action
		// A non-admin cannot widen the scope by forging organizationId in the
		// query string; both the honest and the forged request must 404.
		for _, query := range []string{"", "organizationId=" + forgedOrgRequestID} {
			query := query
			t.Run(action.name+"/query="+query, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				repo := tenantByIDFixture()
				router := tenantByIDRouter(ctrl, repo)

				recorder := doTenantByID(t, router, action, query, true)

				require.Equal(t, http.StatusNotFound, recorder.Code, "body=%s", recorder.Body.String())
				assert.Zero(t, repo.systemLookups, "scoped request paths must not fall back to the system lookup")
				assert.Zero(t, repo.siteUpdates+repo.siteDeletes+repo.serverUpdates+repo.serverDeletes,
					"foreign row must not be mutated")
			})
		}
	}
}

// TestByIDRoutes_EmptyTrustedTenantRejected pins the fail-fast: when the
// authenticated scope carries no tenant, every by-id route must answer 400
// before decoding the body or touching the repository. The malformed body on
// PATCH proves the tenant check runs before JSON decoding.
func TestByIDRoutes_EmptyTrustedTenantRejected(t *testing.T) {
	for _, action := range tenantByIDActions() {
		action := action
		t.Run(action.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := tenantByIDFixture()
			router := tenantByIDRouter(ctrl, repo)

			if action.method == http.MethodPatch {
				action.body = `{"name":` // malformed on purpose
			}
			recorder := doTenantByID(t, router, action, "", false)

			require.Equal(t, http.StatusBadRequest, recorder.Code, "body=%s", recorder.Body.String())
			assert.Contains(t, recorder.Body.String(), "organizationId is required",
				"tenant guard must run before body decoding: %s", recorder.Body.String())
			assert.Zero(t, repo.systemLookups, "no lookup may run without a trusted tenant")
			assert.Zero(t, repo.siteUpdates+repo.siteDeletes+repo.serverUpdates+repo.serverDeletes,
				"no mutation may run without a trusted tenant")
		})
	}
}

// TestByIDRoutes_PlatformAdminExplicitTenantAllowed pins the platform
// administrator escape hatch: with a super-admin scope the explicit
// organizationId targets tenant B and the same eight routes still answer 200.
func TestByIDRoutes_PlatformAdminExplicitTenantAllowed(t *testing.T) {
	for _, action := range tenantByIDActions() {
		action := action
		t.Run(action.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := tenantByIDFixture()
			router := tenantByIDRouter(ctrl, repo)

			var body *strings.Reader
			if action.body != "" {
				body = strings.NewReader(action.body)
			}
			target := action.path + "?organizationId=" + forgedOrgRequestID
			var req *http.Request
			if body != nil {
				req = httptest.NewRequest(action.method, target, body)
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(action.method, target, nil)
			}
			ctx := tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin)
			ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
				UserID: "33333333-3333-3333-3333-333333333333",
			})
			req = req.WithContext(ctx)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
		})
	}
}

// TestByIDRoutes_SameTenantAllowed guards against over-correction: tenant A
// still reads and mutates its own rows through the same eight routes.
func TestByIDRoutes_SameTenantAllowed(t *testing.T) {
	for _, action := range tenantByIDActions() {
		action := action
		t.Run(action.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := tenantByIDFixture()
			repo.sites["foreign-site"].OrganizationID = forgedOrgSessionID
			repo.servers["foreign-server"].OrganizationID = forgedOrgSessionID
			router := tenantByIDRouter(ctrl, repo)

			// A forged query tenant must not override the authenticated scope.
			recorder := doTenantByID(t, router, action, "organizationId="+forgedOrgRequestID, true)

			require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
		})
	}
}
