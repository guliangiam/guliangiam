package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	appcerts "github.com/guliangiam/guliangiam/internal/app/certs"
	appkeys "github.com/guliangiam/guliangiam/internal/app/keys"
	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	apprebac "github.com/guliangiam/guliangiam/internal/app/rebac"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	certdomain "github.com/guliangiam/guliangiam/internal/domain/certs"
	certmock "github.com/guliangiam/guliangiam/internal/domain/certs/mock"
	keydomain "github.com/guliangiam/guliangiam/internal/domain/keys"
	keymock "github.com/guliangiam/guliangiam/internal/domain/keys/mock"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	providermock "github.com/guliangiam/guliangiam/internal/domain/providers/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	certstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/certs"
	keystransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/keys"
	providerstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/providers"
	userstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/users"
	httpmw "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	platformAdminUserID = "11111111-1111-4111-8111-111111111111"
	tenantAdminUserID   = "22222222-2222-4222-8222-222222222222"
	inactiveAdminUserID = "33333333-3333-4333-8333-333333333333"
	tenantOrgID         = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	foreignOrgID        = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

	platformAdminToken = "platform-admin-session"
	tenantAdminToken   = "tenant-admin-session"
	inactiveAdminToken = "inactive-admin-session"
)

// fakeTokenVerifier stands in for infra/jwt.Issuer: identity comes from claims,
// exactly like the production header-session path (request headers are ignored).
type fakeTokenVerifier map[string]map[string]interface{}

func (f fakeTokenVerifier) ValidateToken(token string) (map[string]interface{}, error) {
	claims, ok := f[token]
	if !ok {
		return nil, errors.New("fake token verifier: unknown token")
	}
	return claims, nil
}

func sessionClaims(userID, username, organizationID string) map[string]interface{} {
	return map[string]interface{}{
		"sub":             userID,
		"username":        username,
		"organization_id": organizationID,
	}
}

var errFakeUnused = errors.New("fake platform admin repository: method unused by principal resolution")

type emptyReBACNamespaceResolver struct{}

func (emptyReBACNamespaceResolver) Resolve(context.Context, string) (string, error) {
	return "", nil
}

// fakePlatformAdminRepo backs the real scopeauthorizer.cachedDBResolver. Only
// GetByUserID participates in principal classification; the write methods exist
// solely to satisfy the port.
type fakePlatformAdminRepo struct {
	admins map[string]*platformadmins.PlatformAdmin
}

func (r fakePlatformAdminRepo) GetByUserID(_ context.Context, userID string) (*platformadmins.PlatformAdmin, error) {
	if admin, ok := r.admins[userID]; ok {
		return admin, nil
	}
	return nil, apperrors.NewNotFound("platform admin not found")
}

func (fakePlatformAdminRepo) GetByID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return nil, errFakeUnused
}

func (fakePlatformAdminRepo) List(context.Context, platformadmins.ListOptions) ([]*platformadmins.PlatformAdmin, int64, error) {
	return nil, 0, errFakeUnused
}

func (fakePlatformAdminRepo) Create(context.Context, *platformadmins.PlatformAdmin) error {
	return errFakeUnused
}

func (fakePlatformAdminRepo) Update(context.Context, *platformadmins.PlatformAdmin) error {
	return errFakeUnused
}

func (fakePlatformAdminRepo) SetActive(context.Context, string, bool) error {
	return errFakeUnused
}

func (fakePlatformAdminRepo) Delete(context.Context, string) error {
	return errFakeUnused
}

type enforceCall struct {
	resource string
	action   string
}

// fakePDP grants every Casbin request. That is the adversarial shape for the
// global routes: a tenant administrator legitimately holding `users:read`
// reaches the handler with a Casbin allow, so a 403 can only come from
// RequirePlatformAdmin.
type fakePDP struct {
	enforcerOrgs []string
	calls        []enforceCall
}

func (p *fakePDP) ResolveEnforcerUUID(_ context.Context, organizationID string) (string, error) {
	p.enforcerOrgs = append(p.enforcerOrgs, organizationID)
	return "00000000-0000-4000-8000-000000000001", nil
}

func (p *fakePDP) Enforce(_ context.Context, _, _ string, rvals ...interface{}) (bool, error) {
	call := enforceCall{}
	if len(rvals) == 4 {
		call.resource, _ = rvals[2].(string)
		call.action, _ = rvals[3].(string)
	}
	p.calls = append(p.calls, call)
	return true, nil
}

// fakeAdminScopeAuthorizer 复刻生产链的分类写入：只有显式持有 `all`
// 作用域的平台管理员命中 GrantsAllTenants()，其余一律 ClassNone。
type fakeAdminScopeAuthorizer struct {
	allUserIDs map[string]bool
}

var _ scopeauthorizer.ScopeAuthorizer = fakeAdminScopeAuthorizer{}
var _ scopeauthorizer.AdminScopeResolver = fakeAdminScopeAuthorizer{}

func (a fakeAdminScopeAuthorizer) Authorize(context.Context, scopeauthorizer.AuthorizationRequest) (scopeauthorizer.AuthorizationResult, error) {
	return scopeauthorizer.AuthorizationResult{Allowed: true}, nil
}

func (a fakeAdminScopeAuthorizer) ResolveAdminScope(_ context.Context, userID string) (scopeauthorizer.AdminScope, error) {
	if a.allUserIDs[userID] {
		return scopeauthorizer.AdminScope{Class: scopeauthorizer.AdminScopeClassAll}, nil
	}
	return scopeauthorizer.AdminScope{Class: scopeauthorizer.AdminScopeClassNone}, nil
}

// globalChainRouter assembles one /api/v1/global/* read path with the real
// session middleware, the caller's PrincipalTypeResolver and the real route
// registration. Only the JWT verifier, the platform_admins repository, the
// Casbin PDP and the route's app repository are fakes: no database.
//
// These routes carry no explicit target_tenant, so ScopeAuthorizerMiddleware
// only writes the platform_admin scope classification (annotateAdminScope);
// RLS escalation is still decided inside TenantFilterMiddleware and only for
// an explicit `all` scope, matching the production chain order in chi.go.
func globalChainRouter(
	t *testing.T,
	tokens fakeTokenVerifier,
	resolver scopeauthorizer.PrincipalTypeResolver,
	pdp *fakePDP,
	register func(chi.Router),
) chi.Router {
	t.Helper()

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(authtransport.SessionContextMiddleware(authtransport.Handler{}, tokens, resolver))
	router.Use(authtransport.APIAuthorizationMiddleware(pdp, apprebac.NewEngine(nil), emptyReBACNamespaceResolver{}))
	router.Use(httpmw.ScopeAuthorizerMiddleware(fakeAdminScopeAuthorizer{
		allUserIDs: map[string]bool{platformAdminUserID: true},
	}))
	router.Use(httpmw.TenantFilterMiddleware(httpmw.TenantFilterConfig{StrictMode: false}))
	register(router)
	return router
}

func globalUsersChainRouter(
	t *testing.T,
	tokens fakeTokenVerifier,
	resolver scopeauthorizer.PrincipalTypeResolver,
	pdp *fakePDP,
) (chi.Router, *usermock.MockRepository) {
	t.Helper()

	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockIdentityUoW(ctrl)
	repo := usermock.NewMockRepository(ctrl)
	uow.EXPECT().Users().Return(repo).AnyTimes()

	router := globalChainRouter(t, tokens, resolver, pdp, func(router chi.Router) {
		userstransport.RegisterGlobalChiRoutes(router, userstransport.Handler{
			Service: appusers.Service{UOW: uow},
		})
	})
	return router, repo
}

func globalUsersRequest(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/global/users", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// globalListRoute describes one /api/v1/global/* list route mounted on the
// shared middleware chain. mount installs the real route and returns a closure
// that sets up the route's repository expectation for the platform-admin
// success case. When the closure is not called, any repository access during
// the request is an unexpected gomock call and fails the test.
type globalListRoute struct {
	path     string
	resource string
	mount    func(t *testing.T, tokens fakeTokenVerifier, resolver scopeauthorizer.PrincipalTypeResolver, pdp *fakePDP) (chi.Router, func())
}

func globalListRequest(path, token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// requireCrossTenantListScope pins the RLS escalation the platform admin gate
// must produce: TenantFilterSuperAdmin and no narrowing to the session tenant.
func requireCrossTenantListScope(t *testing.T, ctx context.Context) {
	t.Helper()
	scope := tenantctx.TenantScopeFromContext(ctx)
	require.Equal(t, tenantctx.TenantFilterSuperAdmin, scope.Mode,
		"platform_admin from platform_admins must escalate the tenant scope")
	require.Equal(t, tenantOrgID, scope.OrganizationID)
}

func globalProvidersChainRouter(
	t *testing.T,
	tokens fakeTokenVerifier,
	resolver scopeauthorizer.PrincipalTypeResolver,
	pdp *fakePDP,
) (chi.Router, func()) {
	t.Helper()

	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := providermock.NewMockRepository(ctrl)
	uow.EXPECT().Providers().Return(repo).AnyTimes()

	router := globalChainRouter(t, tokens, resolver, pdp, func(router chi.Router) {
		providerstransport.RegisterGlobalChiRoutes(router, providerstransport.Handler{
			Service: appproviders.Service{UOW: uow},
		})
	})
	return router, func() {
		rows := []*providerdomain.Provider{
			{ID: "provider-own", OrganizationID: tenantOrgID, Name: "own"},
			{ID: "provider-foreign", OrganizationID: foreignOrgID, Name: "foreign"},
		}
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, query providerdomain.ListQuery) ([]*providerdomain.Provider, int64, error) {
				requireCrossTenantListScope(t, ctx)
				require.Empty(t, query.OrganizationID,
					"the global listing must not be narrowed to the session tenant")
				return rows, int64(len(rows)), nil
			})
	}
}

func globalCertsChainRouter(
	t *testing.T,
	tokens fakeTokenVerifier,
	resolver scopeauthorizer.PrincipalTypeResolver,
	pdp *fakePDP,
) (chi.Router, func()) {
	t.Helper()

	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := certmock.NewMockRepository(ctrl)
	uow.EXPECT().Certs().Return(repo).AnyTimes()

	router := globalChainRouter(t, tokens, resolver, pdp, func(router chi.Router) {
		certstransport.RegisterGlobalChiRoutes(router, certstransport.Handler{
			Service: appcerts.Service{UOW: uow},
		})
	})
	return router, func() {
		rows := []*certdomain.Cert{
			{ID: "cert-own", OrganizationID: tenantOrgID, Name: "own"},
			{ID: "cert-foreign", OrganizationID: foreignOrgID, Name: "foreign"},
		}
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, query certdomain.ListQuery) ([]*certdomain.Cert, int64, error) {
				requireCrossTenantListScope(t, ctx)
				require.Empty(t, query.OrganizationID,
					"the global listing must not be narrowed to the session tenant")
				return rows, int64(len(rows)), nil
			})
	}
}

func globalKeysChainRouter(
	t *testing.T,
	tokens fakeTokenVerifier,
	resolver scopeauthorizer.PrincipalTypeResolver,
	pdp *fakePDP,
) (chi.Router, func()) {
	t.Helper()

	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := keymock.NewMockRepository(ctrl)
	uow.EXPECT().Keys().Return(repo).AnyTimes()

	router := globalChainRouter(t, tokens, resolver, pdp, func(router chi.Router) {
		keystransport.RegisterGlobalChiRoutes(router, keystransport.Handler{
			Service: appkeys.Service{UOW: uow},
		})
	})
	return router, func() {
		rows := []*keydomain.Key{
			{ID: "key-own", OrganizationID: tenantOrgID, Name: "own", IsEnabled: true},
			{ID: "key-foreign", OrganizationID: foreignOrgID, Name: "foreign", IsEnabled: true},
		}
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, query keydomain.ListQuery) ([]*keydomain.Key, int64, error) {
				requireCrossTenantListScope(t, ctx)
				require.Empty(t, query.OrganizationID,
					"the global listing must not be narrowed to the session tenant")
				return rows, int64(len(rows)), nil
			})
	}
}

func requireErrorEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	require.Equal(t, wantStatus, recorder.Code)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, wantCode, body.Error.Code)
}

func TestGlobalUsersPlatformAdminGateChain(t *testing.T) {
	resolver := scopeauthorizer.NewCachedDBResolver(fakePlatformAdminRepo{
		admins: map[string]*platformadmins.PlatformAdmin{
			platformAdminUserID: {ID: "pa-active", UserID: platformAdminUserID, IsActive: true},
			inactiveAdminUserID: {ID: "pa-inactive", UserID: inactiveAdminUserID, IsActive: false},
		},
	}, time.Now)
	tokens := fakeTokenVerifier{
		platformAdminToken: sessionClaims(platformAdminUserID, "platform-admin", tenantOrgID),
		tenantAdminToken:   sessionClaims(tenantAdminUserID, "tenant-admin", tenantOrgID),
		inactiveAdminToken: sessionClaims(inactiveAdminUserID, "inactive-admin", tenantOrgID),
	}

	t.Run("tenant administrator holding users:read is rejected", func(t *testing.T) {
		pdp := &fakePDP{}
		router, _ := globalUsersChainRouter(t, tokens, resolver, pdp)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, globalUsersRequest(tenantAdminToken))

		requireErrorEnvelope(t, recorder, http.StatusForbidden, "forbidden")
		// Casbin already allowed the tenant-level `users:read`; the 403 is
		// therefore produced by RequirePlatformAdmin, not by the PEP.
		require.Len(t, pdp.calls, 1)
		assert.Equal(t, "users", pdp.calls[0].resource)
		assert.Equal(t, []string{tenantOrgID}, pdp.enforcerOrgs)
		// The users repository is never touched: gomock fails the test on an
		// unexpected Users().List call.
	})

	t.Run("platform administrator aggregates across tenants", func(t *testing.T) {
		pdp := &fakePDP{}
		router, repo := globalUsersChainRouter(t, tokens, resolver, pdp)
		rows := []*userdomain.User{
			{ID: "user-own", OrganizationID: tenantOrgID},
			{ID: "user-foreign", OrganizationID: foreignOrgID},
		}
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
				scope := tenantctx.TenantScopeFromContext(ctx)
				assert.Equal(t, tenantctx.TenantFilterSuperAdmin, scope.Mode,
					"platform_admin from platform_admins must escalate the tenant scope")
				assert.Equal(t, tenantOrgID, scope.OrganizationID)
				assert.Empty(t, query.OrganizationID,
					"the global listing must not be narrowed to the session tenant")
				return rows, int64(len(rows)), nil
			})

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, globalUsersRequest(platformAdminToken))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Len(t, pdp.calls, 1)
		assert.Equal(t, "users", pdp.calls[0].resource)

		var response userstransport.ListUsersResponse
		require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))
		require.Len(t, response.Items, 2)
		assert.Equal(t, tenantOrgID, response.Items[0].OrganizationID)
		assert.Equal(t, foreignOrgID, response.Items[1].OrganizationID)
	})

	t.Run("inactive platform administrator is classified as user and rejected", func(t *testing.T) {
		pdp := &fakePDP{}
		router, _ := globalUsersChainRouter(t, tokens, resolver, pdp)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, globalUsersRequest(inactiveAdminToken))

		// The resolver only returns platform_admin for is_active rows, so the
		// gate fail-closes on deactivated administrators.
		requireErrorEnvelope(t, recorder, http.StatusForbidden, "forbidden")
	})

	t.Run("no session is rejected before the gate", func(t *testing.T) {
		pdp := &fakePDP{}
		router, _ := globalUsersChainRouter(t, tokens, resolver, pdp)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, globalUsersRequest(""))

		// Production order: APIAuthorizationMiddleware Gate 3 runs before the
		// route handler, so a session-less caller gets 401 unauthorized.
		// RequirePlatformAdmin alone would answer 403 forbidden (the
		// handler-level test pins that); this asserts the chain's real result.
		requireErrorEnvelope(t, recorder, http.StatusUnauthorized, "unauthorized")
		assert.Empty(t, pdp.calls)
	})
}

// TestGlobalListRoutesPlatformAdminGateChain parameterizes the three global
// listings left untested by E1-L2b. All four global routes share the same
// RequirePlatformAdmin gate, the same PrincipalTypeResolver and the same
// adversarial PDP that allows every Casbin request, so a tenant
// administrator's 403 can only come from the gate.
func TestGlobalListRoutesPlatformAdminGateChain(t *testing.T) {
	resolver := scopeauthorizer.NewCachedDBResolver(fakePlatformAdminRepo{
		admins: map[string]*platformadmins.PlatformAdmin{
			platformAdminUserID: {ID: "pa-active", UserID: platformAdminUserID, IsActive: true},
		},
	}, time.Now)
	tokens := fakeTokenVerifier{
		platformAdminToken: sessionClaims(platformAdminUserID, "platform-admin", tenantOrgID),
		tenantAdminToken:   sessionClaims(tenantAdminUserID, "tenant-admin", tenantOrgID),
	}

	routes := []globalListRoute{
		{path: "/api/v1/global/providers", resource: "providers", mount: globalProvidersChainRouter},
		{path: "/api/v1/global/certs", resource: "certs", mount: globalCertsChainRouter},
		{path: "/api/v1/global/keys", resource: "keys", mount: globalKeysChainRouter},
	}

	for _, route := range routes {
		t.Run(route.path, func(t *testing.T) {
			t.Run("tenant administrator holding the read permission is rejected", func(t *testing.T) {
				pdp := &fakePDP{}
				router, _ := route.mount(t, tokens, resolver, pdp)

				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, globalListRequest(route.path, tenantAdminToken))

				requireErrorEnvelope(t, recorder, http.StatusForbidden, "forbidden")
				// Casbin allowed the tenant-level read; the 403 is therefore
				// produced by RequirePlatformAdmin, not by the PEP. The route
				// repository is never touched (gomock fails on unexpected List).
				require.Len(t, pdp.calls, 1)
				assert.Equal(t, route.resource, pdp.calls[0].resource)
				assert.Equal(t, []string{tenantOrgID}, pdp.enforcerOrgs)
			})

			t.Run("platform administrator is not blocked and aggregates", func(t *testing.T) {
				pdp := &fakePDP{}
				router, expectList := route.mount(t, tokens, resolver, pdp)
				expectList()

				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, globalListRequest(route.path, platformAdminToken))

				require.Equal(t, http.StatusOK, recorder.Code)
				require.Len(t, pdp.calls, 1)
				assert.Equal(t, route.resource, pdp.calls[0].resource)

				var response struct {
					Items []struct {
						OrganizationID string `json:"organizationId"`
					} `json:"items"`
				}
				require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))
				require.Len(t, response.Items, 2)
				assert.Equal(t, tenantOrgID, response.Items[0].OrganizationID)
				assert.Equal(t, foreignOrgID, response.Items[1].OrganizationID)
			})
		})
	}
}
