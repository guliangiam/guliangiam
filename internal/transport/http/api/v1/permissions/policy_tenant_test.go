package permissions

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	apppermissions "github.com/guliangiam/guliangiam/internal/app/permissions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type policyTenantUOW struct {
	appuow.UnitOfWork
	permissions domain.Repository
	enforcers   enforcerdomain.Repository
	models      modeldomain.Repository
}

func (u policyTenantUOW) Permissions() domain.Repository       { return u.permissions }
func (u policyTenantUOW) Enforcers() enforcerdomain.Repository { return u.enforcers }
func (u policyTenantUOW) Models() modeldomain.Repository       { return u.models }

type policyTenantCasbin struct {
	domain.CasbinPolicyManager
	reads  int
	writes int
}

func (c *policyTenantCasbin) GetPolicies(string, string) ([][]string, error) {
	c.reads++
	return [][]string{{"p", "alice", "data1", "read"}}, nil
}

func (c *policyTenantCasbin) RemoveFilteredPolicy(string, string, int, ...string) error {
	c.writes++
	return nil
}

func (c *policyTenantCasbin) AddPolicy(string, string, []string) error {
	c.writes++
	return nil
}

func (c *policyTenantCasbin) GetAllObjects(string, string) ([]string, error) {
	c.reads++
	return []string{"data1"}, nil
}

func (c *policyTenantCasbin) GetAllActions(string, string) ([]string, error) {
	c.reads++
	return []string{"read"}, nil
}

func (c *policyTenantCasbin) GetAllRoles(string, string) ([]string, error) {
	c.reads++
	return []string{"admin"}, nil
}

type policyTenantHarness struct {
	repo      *idorPermissionRepo
	enforcers *enforceScopedEnforcerRepo
	models    *enforceScopedModelRepo
	casbin    *policyTenantCasbin
	service   apppermissions.Service
}

func newPolicyTenantHarness() *policyTenantHarness {
	repo := &idorPermissionRepo{permissions: []*domain.Permission{{
		ID: idorPermission, OrganizationID: enforceOwnOrg, Resources: []string{enforceOwnEnforcer},
	}}}
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
	casbin := &policyTenantCasbin{}
	return &policyTenantHarness{
		repo:      repo,
		enforcers: enforcers,
		models:    models,
		casbin:    casbin,
		service: apppermissions.Service{
			UOW: policyTenantUOW{
				permissions: repo,
				enforcers:   enforcers,
				models:      models,
			},
			CasbinManager: casbin,
		},
	}
}

func (h *policyTenantHarness) router() chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: h.service})
	return router
}

func policyTenantRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), enforceOwnOrg, "own-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "11111111-1111-1111-1111-111111111111", OrganizationID: enforceOwnOrg,
		PrincipalType: "user",
	})
	return req.WithContext(ctx)
}

func servePolicy(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func (h *policyTenantHarness) assertZeroPolicyAccess(t *testing.T) {
	t.Helper()
	assert.Zero(t, h.enforcers.unscopedCalls, "unscoped enforcer lookup must never be used")
	assert.Zero(t, h.models.unscopedCalls, "unscoped model lookup must never be used")
	assert.Zero(t, h.casbin.reads, "casbin policy must not be read")
	assert.Zero(t, h.casbin.writes, "casbin policy must not be written")
}

type policyEndpoint struct {
	name   string
	method string
	path   string
	body   string
}

var policyEndpoints = []policyEndpoint{
	{"get-policy", http.MethodGet, "/api/v1/permissions/" + idorPermission + "/policy", ""},
	{"put-policy", http.MethodPut, "/api/v1/permissions/" + idorPermission + "/policy",
		`{"policies":[{"pType":"p","rules":["alice","data1","read"]},{"pType":"p","rules":["bob","data2","write"]}]}`},
	{"objects", http.MethodGet, "/api/v1/permissions/" + idorPermission + "/objects", ""},
	{"actions", http.MethodGet, "/api/v1/permissions/" + idorPermission + "/actions", ""},
	{"roles", http.MethodGet, "/api/v1/permissions/" + idorPermission + "/roles", ""},
}

func TestPolicyEndpoints_WithoutTrustedTenantRejected(t *testing.T) {
	for _, endpoint := range policyEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			h := newPolicyTenantHarness()
			recorder := servePolicy(h.router(), httptest.NewRequest(
				endpoint.method, endpoint.path, strings.NewReader(endpoint.body)))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			h.assertZeroPolicyAccess(t)
			assert.Empty(t, h.repo.scopedOrgs)
			assert.Empty(t, h.enforcers.scopedOrgArgs)
			assert.Empty(t, h.models.scopedOrgArgs)
		})
	}
}

func TestPolicyEndpoints_ForeignTenantEnforcerIsNotFound(t *testing.T) {
	for _, endpoint := range policyEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			h := newPolicyTenantHarness()
			h.repo.permissions[0].Resources = []string{enforceForeignEnf}

			recorder := servePolicy(h.router(), policyTenantRequest(endpoint.method, endpoint.path, endpoint.body))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Equal(t, []string{enforceOwnOrg}, h.repo.scopedOrgs,
				"the permission must be resolved under the authenticated tenant")
			assert.Equal(t, []string{enforceOwnOrg}, h.enforcers.scopedOrgArgs,
				"the referenced enforcer must be resolved under the authenticated tenant")
			assert.Empty(t, h.models.scopedOrgArgs, "a foreign enforcer must not resolve a model")
			h.assertZeroPolicyAccess(t)
		})
	}
}

func TestPolicyEndpoints_MissingEnforcerIsNotFound(t *testing.T) {
	for _, endpoint := range policyEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			h := newPolicyTenantHarness()
			// idorModel is a valid UUID that is not an enforcer row.
			h.repo.permissions[0].Resources = []string{idorModel}

			recorder := servePolicy(h.router(), policyTenantRequest(endpoint.method, endpoint.path, endpoint.body))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Equal(t, []string{enforceOwnOrg}, h.enforcers.scopedOrgArgs)
			assert.Empty(t, h.models.scopedOrgArgs)
			h.assertZeroPolicyAccess(t)
		})
	}
}

func TestPolicyEndpoints_NonEnforcerResourceIsNotFound(t *testing.T) {
	// Permission resources are generic names in normal writes (built-in seeds
	// use "*"). Only the policy endpoint's legacy resources[0] slot is treated
	// as an enforcer link; a non-UUID value cannot be an enforcer PK.
	for _, endpoint := range policyEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			h := newPolicyTenantHarness()
			h.repo.permissions[0].Resources = []string{"*"}

			recorder := servePolicy(h.router(), policyTenantRequest(endpoint.method, endpoint.path, endpoint.body))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Empty(t, h.enforcers.scopedOrgArgs, "a non-enforcer value must not reach the repository")
			assert.Empty(t, h.models.scopedOrgArgs)
			h.assertZeroPolicyAccess(t)
		})
	}
}

func TestPolicyEndpoints_ForeignTenantModelIsNotFound(t *testing.T) {
	for _, endpoint := range []policyEndpoint{policyEndpoints[0], policyEndpoints[1]} {
		t.Run(endpoint.name, func(t *testing.T) {
			h := newPolicyTenantHarness()
			h.enforcers.models[enforceOwnEnforcer] = enforceForeignModel
			h.models.owners[enforceForeignModel] = enforceForeignOrg

			recorder := servePolicy(h.router(), policyTenantRequest(endpoint.method, endpoint.path, endpoint.body))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Equal(t, []string{enforceOwnOrg}, h.models.scopedOrgArgs,
				"the model must be resolved under the authenticated tenant")
			h.assertZeroPolicyAccess(t)
		})
	}
}

func TestPolicyEndpoints_OwnTenantSucceeds(t *testing.T) {
	for _, endpoint := range policyEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			h := newPolicyTenantHarness()

			recorder := servePolicy(h.router(), policyTenantRequest(endpoint.method, endpoint.path, endpoint.body))

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, []string{enforceOwnOrg}, h.repo.scopedOrgs)
			assert.Equal(t, []string{enforceOwnOrg}, h.enforcers.scopedOrgArgs)
			assert.Equal(t, []string{enforceOwnOrg}, h.models.scopedOrgArgs)
			assert.Zero(t, h.enforcers.unscopedCalls)
			assert.Zero(t, h.models.unscopedCalls)
			if endpoint.name == "put-policy" {
				assert.Equal(t, 3, h.casbin.writes, "one clear plus two batch rules")
				assert.Zero(t, h.casbin.reads)
			} else {
				assert.Equal(t, 1, h.casbin.reads)
				assert.Zero(t, h.casbin.writes)
			}
		})
	}
}
