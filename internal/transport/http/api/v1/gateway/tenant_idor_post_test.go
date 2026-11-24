package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	appgateway "github.com/guliangiam/guliangiam/internal/app/gateway"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// tenantSiteRepo is an in-memory gateway repository for the IDOR regression
// tests. It deliberately exposes both the system-level and the tenant-scoped
// site lookups so the same test code compiles and runs before and after the
// transport/app binding change: pre-fix the service calls GetSiteByID (foreign
// site leaks), post-fix it must call GetSiteByIDForOrganization (not found).
type tenantSiteRepo struct {
	sites       map[string]*domain.Site
	siteUpdates int
}

func (r *tenantSiteRepo) GetServerByID(context.Context, string) (*domain.Server, error) {
	return nil, apperrors.NewNotFound("server not found")
}

func (r *tenantSiteRepo) GetServerByIDForOrganization(context.Context, string, string) (*domain.Server, error) {
	return nil, apperrors.NewNotFound("server not found")
}

func (r *tenantSiteRepo) ListServers(context.Context, domain.ListServersQuery) ([]*domain.Server, int64, error) {
	return nil, 0, nil
}

func (r *tenantSiteRepo) CreateServer(context.Context, *domain.Server) error { return nil }

func (r *tenantSiteRepo) UpdateServer(context.Context, *domain.Server) error { return nil }

func (r *tenantSiteRepo) DeleteServer(context.Context, string) error { return nil }

func (r *tenantSiteRepo) GetSiteByID(_ context.Context, id string) (*domain.Site, error) {
	site, ok := r.sites[id]
	if !ok {
		return nil, apperrors.NewNotFound("site not found")
	}
	return site, nil
}

func (r *tenantSiteRepo) GetSiteByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Site, error) {
	site, ok := r.sites[id]
	if !ok || site.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("site not found")
	}
	return site, nil
}

func (r *tenantSiteRepo) ListSites(context.Context, domain.ListSitesQuery) ([]*domain.Site, int64, error) {
	return nil, 0, nil
}

func (r *tenantSiteRepo) CreateSite(context.Context, *domain.Site) error { return nil }

func (r *tenantSiteRepo) UpdateSite(context.Context, *domain.Site) error {
	r.siteUpdates++
	return nil
}

func (r *tenantSiteRepo) DeleteSite(context.Context, string) error { return nil }

type tenantSiteAction struct {
	name string
	path string
	site domain.Site
}

func tenantSiteActions() []tenantSiteAction {
	return []tenantSiteAction{
		{
			name: "provision-cert",
			path: "/api/v1/gateway/sites/foreign-site/provision-cert",
			site: domain.Site{ID: "foreign-site", OrganizationID: forgedOrgRequestID, Host: "127.0.0.1:1"},
		},
		{
			name: "renew-cert",
			path: "/api/v1/gateway/sites/foreign-site/renew-cert",
			site: domain.Site{ID: "foreign-site", OrganizationID: forgedOrgRequestID, Host: "127.0.0.1:1", CertID: "cert-foreign"},
		},
		{
			name: "check-health",
			path: "/api/v1/gateway/sites/foreign-site/check-health",
			site: domain.Site{ID: "foreign-site", OrganizationID: forgedOrgRequestID, Host: "127.0.0.1:1"},
		},
	}
}

func tenantSiteRouter(ctrl *gomock.Controller, repo domain.Repository) chi.Router {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	uow.EXPECT().Gateway().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appgateway.Service{
		Clock: forgedOrgClock{}, UUID: forgedOrgUUID{}, UOW: uow,
	}})
	return router
}

// TestPostSiteActions_ForeignSiteHidden pins the P1 IDOR fix: a caller in
// tenant A must not be able to act on tenant B's site by supplying its id
// (optionally forging organizationId in the query). Both a foreign site and a
// missing site must surface as 404, never as a successful action.
func TestPostSiteActions_ForeignSiteHidden(t *testing.T) {
	for _, action := range tenantSiteActions() {
		action := action
		t.Run(action.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			site := action.site
			repo := &tenantSiteRepo{sites: map[string]*domain.Site{site.ID: &site}}
			router := tenantSiteRouter(ctrl, repo)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost,
				action.path+"?organizationId="+forgedOrgRequestID, nil))

			require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
			assert.Zero(t, repo.siteUpdates, "foreign site must not be mutated")
		})
	}
}

// TestPostSiteActions_EmptyTrustedTenantRejected pins fail-fast behaviour when
// the authenticated scope has no tenant: the request must be rejected with
// 400 before any site lookup or external action.
func TestPostSiteActions_EmptyTrustedTenantRejected(t *testing.T) {
	for _, action := range tenantSiteActions() {
		action := action
		t.Run(action.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			site := action.site
			repo := &tenantSiteRepo{sites: map[string]*domain.Site{site.ID: &site}}
			router := tenantSiteRouter(ctrl, repo)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, action.path, nil))

			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			assert.Zero(t, repo.siteUpdates, "no lookup/action may run without a trusted tenant")
		})
	}
}

// TestPostSiteActions_PlatformAdminExplicitTenantAllowed pins the platform
// administrator escape hatch: an explicit organizationId may target another
// tenant when the scope is super-admin/system.
func TestPostSiteActions_PlatformAdminExplicitTenantAllowed(t *testing.T) {
	for _, action := range tenantSiteActions() {
		action := action
		t.Run(action.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			site := action.site
			repo := &tenantSiteRepo{sites: map[string]*domain.Site{site.ID: &site}}
			router := tenantSiteRouter(ctrl, repo)

			req := httptest.NewRequest(http.MethodPost, action.path+"?organizationId="+forgedOrgRequestID, nil)
			ctx := tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin)
			ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
				UserID: "33333333-3333-3333-3333-333333333333",
			})
			req = req.WithContext(ctx)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Empty(t, recorder.Header().Get("Location"),
				"a non-create POST keeps 200 and must not advertise a Location")
		})
	}
}
