package permissions

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apppermissions "github.com/guliangiam/guliangiam/internal/app/permissions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorSessionOrg = "11111111-1111-1111-1111-111111111111"
	idorForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorPermission = "33333333-3333-3333-3333-333333333333"
	idorEnforcer   = "44444444-4444-4444-4444-444444444444"
	idorModel      = "55555555-5555-5555-5555-555555555555"
)

type idorClock struct{}

func (idorClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorUUID struct{}

func (idorUUID) NewString() (string, error) { return "generated-id", nil }

// idorPermissionRepo is a tenant-aware in-memory repository. A foreign
// permission behaves like a missing one; counters expose whether the write
// path or the unscoped system lookup was reached at all.
type idorPermissionRepo struct {
	permissions  []*domain.Permission
	getByIDCalls int
	scopedOrgs   []string
	updateCalls  int
	deleteCalls  int
}

func (r *idorPermissionRepo) GetByID(_ context.Context, id string) (*domain.Permission, error) {
	r.getByIDCalls++
	if permission := r.find(id, ""); permission != nil {
		return permission, nil
	}
	return nil, apperrors.NewNotFound("permission not found")
}

func (r *idorPermissionRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Permission, error) {
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	if permission := r.find(id, organizationID); permission != nil {
		return permission, nil
	}
	return nil, apperrors.NewNotFound("permission not found")
}

func (r *idorPermissionRepo) find(id, organizationID string) *domain.Permission {
	for _, permission := range r.permissions {
		if permission.ID != id {
			continue
		}
		if organizationID == "" || permission.OrganizationID == organizationID {
			return permission
		}
	}
	return nil
}

func (r *idorPermissionRepo) List(context.Context, domain.ListQuery) ([]*domain.Permission, int64, error) {
	return r.permissions, int64(len(r.permissions)), nil
}

func (r *idorPermissionRepo) Create(context.Context, *domain.Permission) error { return nil }

func (r *idorPermissionRepo) Update(_ context.Context, permission *domain.Permission) error {
	r.updateCalls++
	for i, existing := range r.permissions {
		if existing.ID == permission.ID && existing.OrganizationID == permission.OrganizationID {
			r.permissions[i] = permission
			return nil
		}
	}
	return apperrors.NewNotFound("permission not found")
}

func (r *idorPermissionRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, permission := range r.permissions {
		if permission.ID != id || permission.OrganizationID != organizationID {
			continue
		}
		r.permissions = append(r.permissions[:i], r.permissions[i+1:]...)
		return nil
	}
	return apperrors.NewNotFound("permission not found")
}

type idorEnforcers struct {
	scopedCalls int
}

func (r *idorEnforcers) GetByID(_ context.Context, id string) (*enforcerdomain.Enforcer, error) {
	return &enforcerdomain.Enforcer{ID: id, ModelID: idorModel}, nil
}

func (r *idorEnforcers) GetByIDForOrganization(_ context.Context, id, organizationID string) (*enforcerdomain.Enforcer, error) {
	r.scopedCalls++
	return &enforcerdomain.Enforcer{ID: id, OrganizationID: organizationID, ModelID: idorModel}, nil
}

func (r *idorEnforcers) List(context.Context, enforcerdomain.ListQuery) ([]*enforcerdomain.Enforcer, int64, error) {
	return nil, 0, nil
}

func (r *idorEnforcers) Create(context.Context, *enforcerdomain.Enforcer) error { return nil }

func (r *idorEnforcers) Update(context.Context, *enforcerdomain.Enforcer) error { return nil }

func (r *idorEnforcers) Delete(context.Context, string, string) error { return nil }

type idorModels struct {
	calls int
}

func (m *idorModels) GetByID(_ context.Context, id string) (*modeldomain.Model, error) {
	m.calls++
	return &modeldomain.Model{ID: id, ModelText: "[request_definition]\nr = sub, dom, obj, act"}, nil
}

func (m *idorModels) GetByIDForOrganization(_ context.Context, id, organizationID string) (*modeldomain.Model, error) {
	m.calls++
	return &modeldomain.Model{ID: id, OrganizationID: organizationID, ModelText: "[request_definition]\nr = sub, dom, obj, act"}, nil
}

func (m *idorModels) List(context.Context, modeldomain.ListQuery) ([]*modeldomain.Model, int64, error) {
	return nil, 0, nil
}

func (m *idorModels) Create(context.Context, *modeldomain.Model) error { return nil }

func (m *idorModels) Update(context.Context, *modeldomain.Model) error { return nil }

func (m *idorModels) Delete(context.Context, string, string) error { return nil }

// idorCasbin counts policy reads and writes so a rejected request can be proven
// to have never touched casbin_rules (which has no organization column or RLS).
type idorCasbin struct {
	reads  int
	writes int
}

func (c *idorCasbin) Enforce(context.Context, string, string, ...interface{}) (bool, error) {
	return true, nil
}

func (c *idorCasbin) BatchEnforce(context.Context, string, string, [][]interface{}) ([]bool, error) {
	return nil, nil
}

func (c *idorCasbin) AddPolicy(string, string, []string) error {
	c.writes++
	return nil
}

func (c *idorCasbin) RemovePolicy(string, string, []string) error {
	c.writes++
	return nil
}

func (c *idorCasbin) GetPolicies(string, string) ([][]string, error) {
	c.reads++
	return [][]string{{"p", "alice", "data1", "read"}}, nil
}

func (c *idorCasbin) GetFilteredPolicies(string, string, int, ...string) ([][]string, error) {
	c.reads++
	return [][]string{{"p", "alice", "data1", "read"}}, nil
}

func (c *idorCasbin) UpdatePolicy(string, string, []string, []string) error {
	c.writes++
	return nil
}

func (c *idorCasbin) RemoveFilteredPolicy(string, string, int, ...string) error {
	c.writes++
	return nil
}

func (c *idorCasbin) ReloadPolicy(string, string) error { return nil }

func (c *idorCasbin) GetAllSubjects(string, string) ([]string, error) {
	c.reads++
	return []string{"alice"}, nil
}

func (c *idorCasbin) GetAllObjects(string, string) ([]string, error) {
	c.reads++
	return []string{"data1"}, nil
}

func (c *idorCasbin) GetAllActions(string, string) ([]string, error) {
	c.reads++
	return []string{"read"}, nil
}

func (c *idorCasbin) GetAllRoles(string, string) ([]string, error) {
	c.reads++
	return []string{"admin"}, nil
}

func (c *idorCasbin) AddRoleForUser(string, string, string, string) error { return nil }

func (c *idorCasbin) GetOrgEnforcerPolicies(string) ([][]string, error) { return nil, nil }

func (c *idorCasbin) AddOrgPolicy(string, []string) error { return nil }

func (c *idorCasbin) RemoveOrgFilteredPolicy(string, int, ...string) error { return nil }

func (c *idorCasbin) AddOrgRoleForUser(string, string, string) error { return nil }

func (c *idorCasbin) RemoveOrgFilteredGroupingPolicy(string, int, ...string) error { return nil }

func (c *idorCasbin) InvalidateAll() {}

func (c *idorCasbin) GetOrgSubjectPolicies(string, string) ([][]string, error) { return nil, nil }

type idorRecords struct {
	created []*recordsdomain.Record
}

func (r *idorRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *idorRecords) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *idorRecords) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *idorRecords) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *idorRecords) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *idorRecords) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error { return nil }

func (r *idorRecords) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

type idorUOW struct {
	appuow.UnitOfWork
	permissions domain.Repository
	enforcers   enforcerdomain.Repository
	models      modeldomain.Repository
	records     *idorRecords
}

func (u *idorUOW) Permissions() domain.Repository       { return u.permissions }
func (u *idorUOW) Enforcers() enforcerdomain.Repository { return u.enforcers }
func (u *idorUOW) Models() modeldomain.Repository       { return u.models }
func (u *idorUOW) Records() recordsdomain.Repository    { return u.records }
func (u *idorUOW) Outbox() domainoutbox.Repository      { return nil }
func (u *idorUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type idorHarness struct {
	repo      *idorPermissionRepo
	enforcers *idorEnforcers
	models    *idorModels
	records   *idorRecords
	casbin    *idorCasbin
	router    chi.Router
}

func newIDORHarness(permissions ...*domain.Permission) *idorHarness {
	h := &idorHarness{
		repo:      &idorPermissionRepo{permissions: permissions},
		enforcers: &idorEnforcers{},
		models:    &idorModels{},
		records:   &idorRecords{},
		casbin:    &idorCasbin{},
	}
	service := apppermissions.NewService(idorClock{}, idorUUID{},
		&idorUOW{permissions: h.repo, enforcers: h.enforcers, models: h.models, records: h.records}, h.casbin)
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{Service: service})
	return h
}

func idorRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorSessionOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "66666666-6666-6666-6666-666666666666", OrganizationID: idorSessionOrg,
	})
	return req.WithContext(ctx)
}

func platformAdminRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin))
}

func serveIDOR(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func idorBody(t *testing.T, body string) io.Reader {
	t.Helper()
	if body == "" {
		return nil
	}
	return strings.NewReader(body)
}

var idorPermissionRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"get", http.MethodGet, "/api/v1/permissions/" + idorPermission + "?organizationId=" + idorForeignOrg, ""},
	{"patch", http.MethodPatch, "/api/v1/permissions/" + idorPermission + "?organizationId=" + idorForeignOrg, `{"displayName":"hijacked","revision":1}`},
	{"delete", http.MethodDelete, "/api/v1/permissions/" + idorPermission + "?organizationId=" + idorForeignOrg, ""},
	{"get-policy", http.MethodGet, "/api/v1/permissions/" + idorPermission + "/policy?organizationId=" + idorForeignOrg, ""},
	{"put-policy", http.MethodPut, "/api/v1/permissions/" + idorPermission + "/policy?organizationId=" + idorForeignOrg, `{"policies":[{"pType":"p","rules":["alice","data1","read"]}]}`},
	{"objects", http.MethodGet, "/api/v1/permissions/" + idorPermission + "/objects?organizationId=" + idorForeignOrg, ""},
	{"actions", http.MethodGet, "/api/v1/permissions/" + idorPermission + "/actions?organizationId=" + idorForeignOrg, ""},
	{"roles", http.MethodGet, "/api/v1/permissions/" + idorPermission + "/roles?organizationId=" + idorForeignOrg, ""},
}

func tenantPermission(organizationID string) *domain.Permission {
	return &domain.Permission{
		ID: idorPermission, OrganizationID: organizationID, Name: "tenant-permission",
		DisplayName: "Tenant Permission", Resources: []string{idorEnforcer},
		Actions: []string{"read"}, Domains: []string{"console"}, IsEnabled: true, Revision: 1,
	}
}

// TestPermissionByID_ForeignTenantRejected pins that every by-ID route resolves
// the authenticated tenant, not the caller-supplied organizationId, and that a
// foreign permission is indistinguishable from a missing one: no 403, no write,
// no audit event, and no casbin_rules access (that table has no org column/RLS).
func TestPermissionByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range idorPermissionRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newIDORHarness(tenantPermission(idorForeignOrg))

			recorder := serveIDOR(h.router, idorRequest(tt.method, tt.path, idorBody(t, tt.body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign permission must look missing")
			require.NotEmpty(t, h.repo.scopedOrgs, "ownership must be resolved through the scoped lookup")
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, idorSessionOrg, org, "forged query organizationId must be ignored")
			}
			assert.Zero(t, h.repo.getByIDCalls, "management routes must not use the unscoped system lookup")
			assert.Zero(t, h.repo.updateCalls, "foreign tenant must not update the permission")
			assert.Zero(t, h.repo.deleteCalls, "foreign tenant must not delete the permission")
			assert.Zero(t, h.casbin.reads, "foreign tenant must not read casbin policies")
			assert.Zero(t, h.casbin.writes, "foreign tenant must not write casbin policies")
			assert.Zero(t, h.enforcers.scopedCalls, "foreign tenant must not resolve the enforcer")
			assert.Zero(t, h.models.calls, "foreign tenant must not resolve the casbin model")
			assert.Empty(t, h.records.created, "foreign tenant must not produce an audit event")
			require.Len(t, h.repo.permissions, 1, "foreign permission row must remain untouched")
		})
	}
}

// TestPermissionByID_WithoutTenantScopeRejected pins the fail-fast: when neither
// the session nor a platform-admin target provides a tenant, every by-ID route
// is rejected before touching the repository, the enforcer store or Casbin.
func TestPermissionByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range idorPermissionRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newIDORHarness(tenantPermission(idorForeignOrg))

			req := httptest.NewRequest(tt.method, tt.path, idorBody(t, tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			// Strip the forged query value: an empty trusted org is the failure mode.
			req.URL.RawQuery = ""

			recorder := serveIDOR(h.router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, h.repo.getByIDCalls)
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.deleteCalls)
			assert.Zero(t, h.enforcers.scopedCalls)
			assert.Zero(t, h.models.calls)
			assert.Zero(t, h.casbin.reads)
			assert.Zero(t, h.casbin.writes)
			assert.Empty(t, h.records.created)
		})
	}
}

// TestPermissionByID_OwnTenantSucceeds pins that the eight routes still work
// inside the authenticated tenant, that delete audits the real organization
// (never the built-in fallback), and that policy reads/writes reach Casbin.
func TestPermissionByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := newIDORHarness(tenantPermission(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodGet, "/api/v1/permissions/"+idorPermission, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "tenant-permission")
		require.NotEmpty(t, h.repo.scopedOrgs)
		assert.Equal(t, idorSessionOrg, h.repo.scopedOrgs[0])
		assert.Zero(t, h.repo.getByIDCalls)
	})

	t.Run("patch", func(t *testing.T) {
		h := newIDORHarness(tenantPermission(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodPatch, "/api/v1/permissions/"+idorPermission,
			strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.updateCalls)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorSessionOrg, h.records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		h := newIDORHarness(tenantPermission(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodDelete, "/api/v1/permissions/"+idorPermission, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.deleteCalls)
		assert.Empty(t, h.repo.permissions)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorSessionOrg, h.records.created[0].OrganizationID,
			"delete audit must carry the permission organization, not the built-in fallback")
	})

	t.Run("get-policy", func(t *testing.T) {
		h := newIDORHarness(tenantPermission(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodGet, "/api/v1/permissions/"+idorPermission+"/policy", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.casbin.reads)
		assert.Zero(t, h.casbin.writes)
		assert.Contains(t, recorder.Body.String(), "alice")
	})

	t.Run("put-policy", func(t *testing.T) {
		h := newIDORHarness(tenantPermission(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodPut, "/api/v1/permissions/"+idorPermission+"/policy",
			strings.NewReader(`{"policies":[{"pType":"p","rules":["alice","data1","read"]}]}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.GreaterOrEqual(t, h.casbin.writes, 2, "clear + add must reach Casbin")
	})

	for _, route := range []struct {
		name string
		path string
	}{
		{"objects", "/objects"},
		{"actions", "/actions"},
		{"roles", "/roles"},
	} {
		t.Run(route.name, func(t *testing.T) {
			h := newIDORHarness(tenantPermission(idorSessionOrg))
			recorder := serveIDOR(h.router, idorRequest(http.MethodGet,
				"/api/v1/permissions/"+idorPermission+route.path, nil))

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, 1, h.casbin.reads)
			assert.Zero(t, h.casbin.writes)
		})
	}
}

// TestPermissionByID_PlatformAdminCanTargetTenant pins that a platform scope
// keeps the explicit cross-tenant target; the requested organization is then
// the repository predicate and the audit organization.
func TestPermissionByID_PlatformAdminCanTargetTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := newIDORHarness(tenantPermission(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodGet,
			"/api/v1/permissions/"+idorPermission+"?organizationId="+idorForeignOrg, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.NotEmpty(t, h.repo.scopedOrgs)
		assert.Equal(t, idorForeignOrg, h.repo.scopedOrgs[0])
	})

	t.Run("patch keeps the target tenant", func(t *testing.T) {
		h := newIDORHarness(tenantPermission(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodPatch,
			"/api/v1/permissions/"+idorPermission+"?organizationId="+idorForeignOrg,
			strings.NewReader(`{"displayName":"admin-renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.updateCalls)
		require.Len(t, h.repo.permissions, 1)
		assert.Equal(t, idorForeignOrg, h.repo.permissions[0].OrganizationID)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorForeignOrg, h.records.created[0].OrganizationID)
	})

	t.Run("delete audits the target tenant", func(t *testing.T) {
		h := newIDORHarness(tenantPermission(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodDelete,
			"/api/v1/permissions/"+idorPermission+"?organizationId="+idorForeignOrg, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorForeignOrg, h.records.created[0].OrganizationID)
	})

	t.Run("empty target rejected", func(t *testing.T) {
		h := newIDORHarness(tenantPermission(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodGet,
			"/api/v1/permissions/"+idorPermission, nil))

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Zero(t, h.repo.getByIDCalls)
		assert.Zero(t, h.casbin.reads)
	})
}
