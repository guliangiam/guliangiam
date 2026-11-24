package permissions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apppermissions "github.com/guliangiam/guliangiam/internal/app/permissions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	ctodomain "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	permissiondomain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	httpmiddleware "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	enforceOwnOrg        = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	enforceForeignOrg    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	enforceOwnEnforcer   = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	enforceForeignEnf    = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	enforceOwnModel      = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	enforceForeignModel  = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	enforceAdminUserID   = "99999999-9999-9999-9999-999999999999"
	enforceAdminRecordID = "77777777-7777-7777-7777-777777777777"
)

var enforcePaths = []struct {
	name string
	path string
	body func(enforcerID string) string
}{
	{
		name: "enforce",
		path: "/api/v1/permissions/enforce",
		body: func(id string) string {
			return `{"enforcerId":"` + id + `","sub":"alice","obj":"data","act":"read"}`
		},
	},
	{
		name: "batch-enforce",
		path: "/api/v1/permissions/batch-enforce",
		body: func(id string) string {
			return `{"enforcerId":"` + id + `","requests":[{"sub":"alice","obj":"data","act":"read"}]}`
		},
	},
}

type enforceTenantUOW struct {
	appuow.UnitOfWork
	enforcers enforcerdomain.Repository
	models    modeldomain.Repository
}

func (u enforceTenantUOW) Enforcers() enforcerdomain.Repository { return u.enforcers }
func (u enforceTenantUOW) Models() modeldomain.Repository       { return u.models }

type enforceScopedEnforcerRepo struct {
	enforcerdomain.Repository
	owners        map[string]string
	models        map[string]string
	unscopedCalls int
	scopedOrgArgs []string
}

func (r *enforceScopedEnforcerRepo) GetByID(context.Context, string) (*enforcerdomain.Enforcer, error) {
	r.unscopedCalls++
	return nil, apperrors.NewNotFound("enforcer not found")
}

func (r *enforceScopedEnforcerRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*enforcerdomain.Enforcer, error) {
	r.scopedOrgArgs = append(r.scopedOrgArgs, organizationID)
	if owner, ok := r.owners[id]; ok && owner == organizationID {
		return &enforcerdomain.Enforcer{ID: id, OrganizationID: organizationID, ModelID: r.models[id]}, nil
	}
	return nil, apperrors.NewNotFound("enforcer not found")
}

type enforceScopedModelRepo struct {
	modeldomain.Repository
	owners        map[string]string
	unscopedCalls int
	scopedOrgArgs []string
}

func (r *enforceScopedModelRepo) GetByID(context.Context, string) (*modeldomain.Model, error) {
	r.unscopedCalls++
	return nil, apperrors.NewNotFound("model not found")
}

func (r *enforceScopedModelRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*modeldomain.Model, error) {
	r.scopedOrgArgs = append(r.scopedOrgArgs, organizationID)
	if owner, ok := r.owners[id]; ok && owner == organizationID {
		return &modeldomain.Model{
			ID: id, OrganizationID: organizationID,
			ModelText: "[request_definition]\nr = sub, dom, obj, act",
		}, nil
	}
	return nil, apperrors.NewNotFound("model not found")
}

type enforceCountingCasbin struct {
	permissiondomain.CasbinPolicyManager
	enforceCalls int
	batchCalls   int
}

func (c *enforceCountingCasbin) Enforce(context.Context, string, string, ...interface{}) (bool, error) {
	c.enforceCalls++
	return true, nil
}

func (c *enforceCountingCasbin) BatchEnforce(context.Context, string, string, [][]interface{}) ([]bool, error) {
	c.batchCalls++
	return []bool{true}, nil
}

type enforceTenantHarness struct {
	enforcers *enforceScopedEnforcerRepo
	models    *enforceScopedModelRepo
	casbin    *enforceCountingCasbin
	service   apppermissions.Service
}

func newEnforceTenantHarness() *enforceTenantHarness {
	enforcers := &enforceScopedEnforcerRepo{
		owners: map[string]string{
			enforceOwnEnforcer: enforceOwnOrg,
			enforceForeignEnf:  enforceForeignOrg,
		},
		models: map[string]string{
			enforceOwnEnforcer: enforceOwnModel,
			enforceForeignEnf:  enforceForeignModel,
		},
	}
	models := &enforceScopedModelRepo{owners: map[string]string{
		enforceOwnModel:     enforceOwnOrg,
		enforceForeignModel: enforceForeignOrg,
	}}
	casbin := &enforceCountingCasbin{}
	return &enforceTenantHarness{
		enforcers: enforcers,
		models:    models,
		casbin:    casbin,
		service: apppermissions.Service{
			UOW:           enforceTenantUOW{enforcers: enforcers, models: models},
			CasbinManager: casbin,
		},
	}
}

// router optionally wires the real tenant/scope middleware chain. Passing nil
// exercises the handler's own trusted-tenant resolution.
func (h *enforceTenantHarness) router(authorizer scopeauthorizer.ScopeAuthorizer) chi.Router {
	router := chi.NewRouter()
	if authorizer != nil {
		router.Use(httpmiddleware.TenantResolutionMiddleware())
		router.Use(httpmiddleware.ScopeAuthorizerMiddleware(authorizer))
	}
	RegisterChiRoutes(router, Handler{Service: h.service})
	return router
}

func tenantEnforceRequest(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), enforceOwnOrg, "own-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "11111111-1111-1111-1111-111111111111", OrganizationID: enforceOwnOrg,
		PrincipalType: "user",
	})
	return req.WithContext(ctx)
}

func platformAdminEnforceRequest(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := authtransport.WithSessionContext(req.Context(), authtransport.SessionContext{
		UserID: enforceAdminUserID, OrganizationID: enforceOwnOrg,
		PrincipalType: "platform_admin",
	})
	return req.WithContext(ctx)
}

func serveEnforce(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func (h *enforceTenantHarness) assertZeroPolicyAccess(t *testing.T) {
	t.Helper()
	assert.Zero(t, h.enforcers.unscopedCalls, "unscoped enforcer lookup must never be used")
	assert.Zero(t, h.models.unscopedCalls, "unscoped model lookup must never be used")
	assert.Zero(t, h.casbin.enforceCalls, "casbin policy must not be read")
	assert.Zero(t, h.casbin.batchCalls, "casbin policy must not be read")
}

func TestEnforce_WithoutTrustedTenantRejected(t *testing.T) {
	for _, tt := range enforcePaths {
		t.Run(tt.name, func(t *testing.T) {
			h := newEnforceTenantHarness()
			recorder := serveEnforce(h.router(nil), httptest.NewRequest(
				http.MethodPost, tt.path, strings.NewReader(tt.body(enforceOwnEnforcer))))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			h.assertZeroPolicyAccess(t)
			assert.Empty(t, h.enforcers.scopedOrgArgs)
			assert.Empty(t, h.models.scopedOrgArgs)
		})
	}
}

func TestEnforce_ForeignTenantEnforcerIsNotFound(t *testing.T) {
	for _, tt := range enforcePaths {
		t.Run(tt.name, func(t *testing.T) {
			h := newEnforceTenantHarness()
			recorder := serveEnforce(h.router(nil), tenantEnforceRequest(tt.path, tt.body(enforceForeignEnf)))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Equal(t, []string{enforceOwnOrg}, h.enforcers.scopedOrgArgs,
				"ownership must be resolved under the authenticated tenant")
			assert.Empty(t, h.models.scopedOrgArgs, "a foreign enforcer must not resolve a model")
			h.assertZeroPolicyAccess(t)
		})
	}
}

func TestEnforce_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	for _, tt := range enforcePaths {
		t.Run(tt.name, func(t *testing.T) {
			h := newEnforceTenantHarness()
			path := tt.path + "?organizationId=" + enforceForeignOrg
			recorder := serveEnforce(h.router(nil), tenantEnforceRequest(path, tt.body(enforceForeignEnf)))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Equal(t, []string{enforceOwnOrg}, h.enforcers.scopedOrgArgs,
				"a regular tenant cannot redirect the lookup with the query value")
			h.assertZeroPolicyAccess(t)
		})
	}
}

func TestEnforce_ForeignTenantModelIsNotFound(t *testing.T) {
	h := newEnforceTenantHarness()
	// The enforcer is owned by the caller's tenant, but its model reference
	// points at another tenant. The model lookup must still be scoped.
	h.enforcers.models[enforceOwnEnforcer] = enforceForeignModel

	recorder := serveEnforce(h.router(nil), tenantEnforceRequest(
		"/api/v1/permissions/enforce", enforcePaths[0].body(enforceOwnEnforcer)))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, []string{enforceOwnOrg}, h.models.scopedOrgArgs)
	h.assertZeroPolicyAccess(t)
}

func TestEnforce_OwnTenantSucceeds(t *testing.T) {
	for _, tt := range enforcePaths {
		t.Run(tt.name, func(t *testing.T) {
			h := newEnforceTenantHarness()
			recorder := serveEnforce(h.router(nil), tenantEnforceRequest(tt.path, tt.body(enforceOwnEnforcer)))

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, []string{enforceOwnOrg}, h.enforcers.scopedOrgArgs)
			assert.Equal(t, []string{enforceOwnOrg}, h.models.scopedOrgArgs)
			assert.Zero(t, h.enforcers.unscopedCalls)
			assert.Zero(t, h.models.unscopedCalls)
			if tt.name == "enforce" {
				assert.Equal(t, 1, h.casbin.enforceCalls)
				var result struct {
					Allowed bool `json:"allowed"`
				}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
				assert.True(t, result.Allowed)
			} else {
				assert.Equal(t, 1, h.casbin.batchCalls)
				var result struct {
					Results []bool `json:"results"`
				}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
				assert.Equal(t, []bool{true}, result.Results)
			}
		})
	}
}

type enforceAdminRepo struct {
	platformadmins.PlatformAdminRepository
	admin *platformadmins.PlatformAdmin
}

func (r enforceAdminRepo) GetByUserID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return r.admin, nil
}

type enforceScopeRepo struct {
	platformadmins.PlatformAdminScopeRepository
	coveredTenants map[string]bool
}

func (r enforceScopeRepo) HasActiveScopeForTenant(_ context.Context, _ string, tenantID string) (bool, error) {
	return r.coveredTenants[tenantID], nil
}

// ListByPlatformAdmin 供 ScopeAuthorizer 放行后的 annotateAuthorizedAdminScope
// 解析作用域快照：把 coveredTenants 映射为一条 targeted 作用域，与
// HasActiveScopeForTenant 的授权口径保持一致。
func (r enforceScopeRepo) ListByPlatformAdmin(_ context.Context, _ string, _ platformadmins.ScopeListOptions) ([]*platformadmins.PlatformAdminScope, error) {
	if len(r.coveredTenants) == 0 {
		return nil, nil
	}
	tenants := make([]string, 0, len(r.coveredTenants))
	for tenant := range r.coveredTenants {
		tenants = append(tenants, tenant)
	}
	return []*platformadmins.PlatformAdminScope{{
		ScopeType:     platformadmins.ScopeTypeTargeted,
		TargetTenants: tenants,
		ExpiresAt:     time.Now().Add(time.Hour),
		IsBootstrap:   true,
	}}, nil
}

type enforceTicketRepo struct {
	ctodomain.TicketRepository
	hasApproved bool
}

func (r enforceTicketRepo) HasApprovedTicket(context.Context, string, string, string, string) (bool, error) {
	return r.hasApproved, nil
}

func newEnforceScopeAuthorizer(coveredTenants map[string]bool, approvedTicket bool) scopeauthorizer.ScopeAuthorizer {
	return scopeauthorizer.NewAllowIfScopedScopeAuthorizer(
		enforceAdminRepo{admin: &platformadmins.PlatformAdmin{
			ID: enforceAdminRecordID, UserID: enforceAdminUserID, IsActive: true,
		}},
		enforceScopeRepo{coveredTenants: coveredTenants},
		enforceTicketRepo{hasApproved: approvedTicket},
		nil, nil,
	)
}

// TestEnforce_PlatformAdminTargetCovered pins that an explicit organizationId
// still works for a platform administrator whose scope covers that tenant: the
// real scope middleware authorizes the target and the handler reads it scoped.
func TestEnforce_PlatformAdminTargetCovered(t *testing.T) {
	for _, tt := range enforcePaths {
		t.Run(tt.name, func(t *testing.T) {
			h := newEnforceTenantHarness()
			authorizer := newEnforceScopeAuthorizer(map[string]bool{enforceForeignOrg: true}, true)
			path := tt.path + "?organizationId=" + enforceForeignOrg
			recorder := serveEnforce(h.router(authorizer), platformAdminEnforceRequest(path, tt.body(enforceForeignEnf)))

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, []string{enforceForeignOrg}, h.enforcers.scopedOrgArgs)
			assert.Equal(t, []string{enforceForeignOrg}, h.models.scopedOrgArgs)
			assert.Zero(t, h.enforcers.unscopedCalls)
			assert.Zero(t, h.models.unscopedCalls)
			if tt.name == "enforce" {
				assert.Equal(t, 1, h.casbin.enforceCalls)
			} else {
				assert.Equal(t, 1, h.casbin.batchCalls)
			}
		})
	}
}

// TestEnforce_PlatformAdminTargetNotCovered pins the deny path: a platform
// administrator without a scope covering the target is rejected by the real
// scope middleware before any enforcer/model/policy access.
func TestEnforce_PlatformAdminTargetNotCovered(t *testing.T) {
	for _, tt := range enforcePaths {
		t.Run(tt.name, func(t *testing.T) {
			h := newEnforceTenantHarness()
			authorizer := newEnforceScopeAuthorizer(map[string]bool{}, true)
			path := tt.path + "?organizationId=" + enforceForeignOrg
			recorder := serveEnforce(h.router(authorizer), platformAdminEnforceRequest(path, tt.body(enforceForeignEnf)))

			assert.Equal(t, http.StatusForbidden, recorder.Code)
			h.assertZeroPolicyAccess(t)
			assert.Empty(t, h.enforcers.scopedOrgArgs)
			assert.Empty(t, h.models.scopedOrgArgs)
		})
	}
}
