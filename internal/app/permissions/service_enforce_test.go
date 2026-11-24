package permissions

import (
	"context"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	permissiondomain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

type mockCasbinManager struct {
	enforceResult      bool
	enforceErr         error
	batchEnforceResult []bool
	batchEnforceErr    error
	policiesResult     [][]string
	policiesErr        error
}

func (m *mockCasbinManager) Enforce(ctx context.Context, enforcerID, modelText string, rvals ...interface{}) (bool, error) {
	return m.enforceResult, m.enforceErr
}

func (m *mockCasbinManager) BatchEnforce(ctx context.Context, enforcerID, modelText string, rvals [][]interface{}) ([]bool, error) {
	return m.batchEnforceResult, m.batchEnforceErr
}

func (m *mockCasbinManager) AddPolicy(enforcerID, modelText string, params []string) error {
	return nil
}

func (m *mockCasbinManager) RemovePolicy(enforcerID, modelText string, params []string) error {
	return nil
}

func (m *mockCasbinManager) GetPolicies(enforcerID, modelText string) ([][]string, error) {
	return m.policiesResult, m.policiesErr
}

func (m *mockCasbinManager) GetFilteredPolicies(enforcerID, modelText string, fieldIndex int, fieldValues ...string) ([][]string, error) {
	return nil, nil
}

func (m *mockCasbinManager) UpdatePolicy(enforcerID, modelText string, oldPolicy, newPolicy []string) error {
	return nil
}

func (m *mockCasbinManager) RemoveFilteredPolicy(enforcerID, modelText string, fieldIndex int, fieldValues ...string) error {
	return nil
}

func (m *mockCasbinManager) ReloadPolicy(enforcerID, modelText string) error {
	return nil
}

func (m *mockCasbinManager) GetAllSubjects(enforcerID, modelText string) ([]string, error) {
	return nil, nil
}

func (m *mockCasbinManager) GetAllObjects(enforcerID, modelText string) ([]string, error) {
	return nil, nil
}

func (m *mockCasbinManager) GetAllActions(enforcerID, modelText string) ([]string, error) {
	return nil, nil
}

func (m *mockCasbinManager) GetAllRoles(enforcerID, modelText string) ([]string, error) {
	return nil, nil
}

func (m *mockCasbinManager) AddRoleForUser(enforcerID, modelText, user, role string) error {
	return nil
}

func (m *mockCasbinManager) GetOrgEnforcerPolicies(orgID string) ([][]string, error) {
	return nil, nil
}

func (m *mockCasbinManager) AddOrgPolicy(orgID string, params []string) error {
	return nil
}

func (m *mockCasbinManager) RemoveOrgFilteredPolicy(orgID string, fieldIndex int, fieldValues ...string) error {
	return nil
}

func (m *mockCasbinManager) AddOrgRoleForUser(orgID, user, role string) error {
	return nil
}

func (m *mockCasbinManager) RemoveOrgFilteredGroupingPolicy(orgID string, fieldIndex int, fieldValues ...string) error {
	return nil
}

func (m *mockCasbinManager) InvalidateAll() {}

func TestEnforce_EmptyEnforcerID(t *testing.T) {
	svc := Service{}
	result, err := svc.Enforce(context.Background(), "org-1", "", EnforceRequest{Sub: "user", Obj: "res", Act: "read"})
	assert.False(t, result)
	assert.NotNil(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func (m *mockCasbinManager) GetOrgSubjectPolicies(orgID, subject string) ([][]string, error) {
	return nil, nil
}

func TestEnforce_WhitespaceEnforcerID(t *testing.T) {
	svc := Service{}
	result, err := svc.Enforce(context.Background(), "org-1", "   ", EnforceRequest{Sub: "user", Obj: "res", Act: "read"})
	assert.False(t, result)
	assert.NotNil(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestBatchEnforce_EmptyEnforcerID(t *testing.T) {
	svc := Service{}
	result, err := svc.BatchEnforce(context.Background(), "org-1", "", []EnforceRequest{{Sub: "user", Obj: "res", Act: "read"}})
	assert.Nil(t, result)
	assert.NotNil(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestBatchEnforce_WhitespaceEnforcerID(t *testing.T) {
	svc := Service{}
	result, err := svc.BatchEnforce(context.Background(), "org-1", "  ", []EnforceRequest{{Sub: "user", Obj: "res", Act: "read"}})
	assert.Nil(t, result)
	assert.NotNil(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestBuildEnforceRvals_WithDomain(t *testing.T) {
	rvals := buildEnforceRvals(EnforceRequest{Sub: "alice", Dom: "org1", Obj: "data1", Act: "read"})
	assert.Equal(t, []interface{}{"alice", "org1", "data1", "read"}, rvals)
}

func TestBuildEnforceRvals_WithoutDomain(t *testing.T) {
	rvals := buildEnforceRvals(EnforceRequest{Sub: "alice", Obj: "data1", Act: "read"})
	assert.Equal(t, []interface{}{"alice", "data1", "read"}, rvals)
}

func TestNormalizeEffect(t *testing.T) {
	assert.Equal(t, "allow", normalizeEffect(""))
	assert.Equal(t, "allow", normalizeEffect("  "))
	assert.Equal(t, "allow", normalizeEffect("allow"))
	assert.Equal(t, "deny", normalizeEffect("Deny"))
	assert.Equal(t, "allow", normalizeEffect("ALLOW"))
}

func TestNormalizeState(t *testing.T) {
	assert.Equal(t, "active", normalizeState(""))
	assert.Equal(t, "active", normalizeState("  "))
	assert.Equal(t, "active", normalizeState("active"))
	assert.Equal(t, "pending", normalizeState("Pending"))
	assert.Equal(t, "approved", normalizeState("APPROVED"))
}

func TestIsValidEffect(t *testing.T) {
	assert.True(t, isValidEffect("allow"))
	assert.True(t, isValidEffect("deny"))
	assert.True(t, isValidEffect("Allow"))
	assert.True(t, isValidEffect("DENY"))
	assert.True(t, isValidEffect(""))
	assert.True(t, isValidEffect("  "))
	assert.False(t, isValidEffect("block"))
}

func TestIsValidState(t *testing.T) {
	assert.True(t, isValidState("active"))
	assert.True(t, isValidState("pending"))
	assert.True(t, isValidState("approved"))
	assert.True(t, isValidState("rejected"))
	assert.True(t, isValidState("disabled"))
	assert.True(t, isValidState(""))
	assert.True(t, isValidState("  "))
	assert.False(t, isValidState("unknown"))
}

func TestSyncPermissionToCasbin_NilManager(t *testing.T) {
	err := syncPermissionToCasbin(nil, "org-1", &permissiondomain.Permission{IsEnabled: true})
	assert.Nil(t, err)
}

func TestSyncPermissionToCasbin_NilPermission(t *testing.T) {
	err := syncPermissionToCasbin(&mockCasbinManager{}, "org-1", nil)
	assert.Nil(t, err)
}

func TestSyncPermissionToCasbin_DisabledPermission(t *testing.T) {
	err := syncPermissionToCasbin(&mockCasbinManager{}, "org-1", &permissiondomain.Permission{IsEnabled: false})
	assert.Nil(t, err)
}

func TestRemovePermissionFromCasbin_NilManager(t *testing.T) {
	err := removePermissionFromCasbin(nil, "org-1", "perm-1")
	assert.Nil(t, err)
}

// scopedEnforceUOW exposes only the enforcer/model ports used by enforce.
type scopedEnforceUOW struct {
	appuow.UnitOfWork
	enforcers enforcerdomain.Repository
	models    modeldomain.Repository
}

func (u scopedEnforceUOW) Enforcers() enforcerdomain.Repository { return u.enforcers }
func (u scopedEnforceUOW) Models() modeldomain.Repository       { return u.models }

// scopedEnforcerRepo is tenant-aware: a row owned by another organization
// behaves like a missing row, and the counters prove which lookup was used.
type scopedEnforcerRepo struct {
	enforcerdomain.Repository
	owners          map[string]string
	modelByEnforcer map[string]string
	unscopedCalls   int
	scopedOrgArgs   []string
}

func (r *scopedEnforcerRepo) GetByID(context.Context, string) (*enforcerdomain.Enforcer, error) {
	r.unscopedCalls++
	return nil, apperrors.NewNotFound("enforcer not found")
}

func (r *scopedEnforcerRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*enforcerdomain.Enforcer, error) {
	r.scopedOrgArgs = append(r.scopedOrgArgs, organizationID)
	if owner, ok := r.owners[id]; ok && owner == organizationID {
		return &enforcerdomain.Enforcer{ID: id, OrganizationID: organizationID, ModelID: r.modelByEnforcer[id]}, nil
	}
	return nil, apperrors.NewNotFound("enforcer not found")
}

type scopedModelRepo struct {
	modeldomain.Repository
	owners        map[string]string
	unscopedCalls int
	scopedOrgArgs []string
}

func (r *scopedModelRepo) GetByID(context.Context, string) (*modeldomain.Model, error) {
	r.unscopedCalls++
	return nil, apperrors.NewNotFound("model not found")
}

func (r *scopedModelRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*modeldomain.Model, error) {
	r.scopedOrgArgs = append(r.scopedOrgArgs, organizationID)
	if owner, ok := r.owners[id]; ok && owner == organizationID {
		return &modeldomain.Model{ID: id, OrganizationID: organizationID, ModelText: "[request_definition]\nr = sub, dom, obj, act"}, nil
	}
	return nil, apperrors.NewNotFound("model not found")
}

type countingEnforceCasbin struct {
	permissiondomain.CasbinPolicyManager
	enforceCalls int
	batchCalls   int
}

func (m *countingEnforceCasbin) Enforce(context.Context, string, string, ...interface{}) (bool, error) {
	m.enforceCalls++
	return true, nil
}

func (m *countingEnforceCasbin) BatchEnforce(context.Context, string, string, [][]interface{}) ([]bool, error) {
	m.batchCalls++
	return []bool{true}, nil
}

func newScopedEnforceService(enforcers *scopedEnforcerRepo, models *scopedModelRepo, casbin *countingEnforceCasbin) Service {
	return Service{
		UOW:           scopedEnforceUOW{enforcers: enforcers, models: models},
		CasbinManager: casbin,
	}
}

func assertInvalidArgument(t *testing.T, err error) {
	t.Helper()
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestEnforce_RequiresOrganization(t *testing.T) {
	svc := Service{}
	_, err := svc.Enforce(context.Background(), " ", "enf-1", EnforceRequest{Sub: "alice", Obj: "data", Act: "read"})
	assertInvalidArgument(t, err)
}

func TestBatchEnforce_RequiresOrganization(t *testing.T) {
	svc := Service{}
	_, err := svc.BatchEnforce(context.Background(), "", "enf-1", []EnforceRequest{{Sub: "alice", Obj: "data", Act: "read"}})
	assertInvalidArgument(t, err)
}

func TestEnforce_ForeignTenantIsNotFound(t *testing.T) {
	enforcers := &scopedEnforcerRepo{
		owners:          map[string]string{"enf-1": "org-2"},
		modelByEnforcer: map[string]string{"enf-1": "model-1"},
	}
	models := &scopedModelRepo{owners: map[string]string{"model-1": "org-2"}}
	casbin := &countingEnforceCasbin{}
	svc := newScopedEnforceService(enforcers, models, casbin)

	allowed, err := svc.Enforce(context.Background(), "org-1", "enf-1", EnforceRequest{Sub: "alice", Obj: "data", Act: "read"})
	assert.False(t, allowed)
	assertNotFound(t, err)
	assert.Equal(t, []string{"org-1"}, enforcers.scopedOrgArgs)
	assert.Zero(t, enforcers.unscopedCalls, "enforce must not use the unscoped lookup")
	assert.Empty(t, models.scopedOrgArgs, "the model must not be read for a foreign enforcer")
	assert.Zero(t, casbin.enforceCalls, "no policy evaluation for a foreign enforcer")
}

func TestBatchEnforce_ForeignTenantIsNotFound(t *testing.T) {
	enforcers := &scopedEnforcerRepo{
		owners:          map[string]string{"enf-1": "org-2"},
		modelByEnforcer: map[string]string{"enf-1": "model-1"},
	}
	models := &scopedModelRepo{owners: map[string]string{"model-1": "org-2"}}
	casbin := &countingEnforceCasbin{}
	svc := newScopedEnforceService(enforcers, models, casbin)

	results, err := svc.BatchEnforce(context.Background(), "org-1", "enf-1",
		[]EnforceRequest{{Sub: "alice", Obj: "data", Act: "read"}})
	assert.Nil(t, results)
	assertNotFound(t, err)
	assert.Equal(t, []string{"org-1"}, enforcers.scopedOrgArgs)
	assert.Zero(t, enforcers.unscopedCalls, "batch enforce must not use the unscoped lookup")
	assert.Empty(t, models.scopedOrgArgs, "the model must not be read for a foreign enforcer")
	assert.Zero(t, casbin.batchCalls, "no policy evaluation for a foreign enforcer")
}

func TestEnforce_ForeignTenantModelIsNotFound(t *testing.T) {
	enforcers := &scopedEnforcerRepo{
		owners:          map[string]string{"enf-1": "org-1"},
		modelByEnforcer: map[string]string{"enf-1": "model-1"},
	}
	models := &scopedModelRepo{owners: map[string]string{"model-1": "org-2"}}
	casbin := &countingEnforceCasbin{}
	svc := newScopedEnforceService(enforcers, models, casbin)

	allowed, err := svc.Enforce(context.Background(), "org-1", "enf-1", EnforceRequest{Sub: "alice", Obj: "data", Act: "read"})
	assert.False(t, allowed)
	assertNotFound(t, err)
	assert.Equal(t, []string{"org-1"}, models.scopedOrgArgs)
	assert.Zero(t, models.unscopedCalls, "the model must be resolved under the trusted tenant")
	assert.Zero(t, casbin.enforceCalls)
}

func TestEnforce_OwnTenantSucceeds(t *testing.T) {
	enforcers := &scopedEnforcerRepo{
		owners:          map[string]string{"enf-1": "org-1"},
		modelByEnforcer: map[string]string{"enf-1": "model-1"},
	}
	models := &scopedModelRepo{owners: map[string]string{"model-1": "org-1"}}
	casbin := &countingEnforceCasbin{}
	svc := newScopedEnforceService(enforcers, models, casbin)

	allowed, err := svc.Enforce(context.Background(), "org-1", "enf-1", EnforceRequest{Sub: "alice", Obj: "data", Act: "read"})
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, []string{"org-1"}, enforcers.scopedOrgArgs)
	assert.Equal(t, []string{"org-1"}, models.scopedOrgArgs)
	assert.Zero(t, enforcers.unscopedCalls)
	assert.Zero(t, models.unscopedCalls)
	assert.Equal(t, 1, casbin.enforceCalls)
}

func TestBatchEnforce_OwnTenantSucceeds(t *testing.T) {
	enforcers := &scopedEnforcerRepo{
		owners:          map[string]string{"enf-1": "org-1"},
		modelByEnforcer: map[string]string{"enf-1": "model-1"},
	}
	models := &scopedModelRepo{owners: map[string]string{"model-1": "org-1"}}
	casbin := &countingEnforceCasbin{}
	svc := newScopedEnforceService(enforcers, models, casbin)

	results, err := svc.BatchEnforce(context.Background(), "org-1", "enf-1",
		[]EnforceRequest{{Sub: "alice", Obj: "data", Act: "read"}})
	assert.NoError(t, err)
	assert.Equal(t, []bool{true}, results)
	assert.Equal(t, []string{"org-1"}, enforcers.scopedOrgArgs)
	assert.Equal(t, []string{"org-1"}, models.scopedOrgArgs)
	assert.Zero(t, enforcers.unscopedCalls)
	assert.Zero(t, models.unscopedCalls)
	assert.Equal(t, 1, casbin.batchCalls)
}
