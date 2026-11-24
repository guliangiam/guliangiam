package permissions

import (
	"context"
	"testing"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"

	actionsdomain "github.com/guliangiam/guliangiam/internal/domain/actions"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	idvdomain "github.com/guliangiam/guliangiam/internal/domain/idv"
	igadomain "github.com/guliangiam/guliangiam/internal/domain/iga"
	meteringdomain "github.com/guliangiam/guliangiam/internal/domain/metering"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	permissiondomain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	riskdomain "github.com/guliangiam/guliangiam/internal/domain/risk"
	workloaddomain "github.com/guliangiam/guliangiam/internal/domain/workload"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// Tests for Service.NewPermission
// ---------------------------------------------------------------------------

func TestService_NewPermission_DefaultEffect(t *testing.T) {
	now := time.Now()
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-perm-id"},
	}

	perm, err := svc.NewPermission(CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "read-users",
		DisplayName:    "Read Users",
		Effect:         "",
	})
	if err != nil {
		t.Fatalf("NewPermission() error = %v", err)
	}
	if perm.Effect != "allow" {
		t.Errorf("Effect = %q, want %q", perm.Effect, "allow")
	}
}

func TestService_NewPermission_DenyEffect(t *testing.T) {
	now := time.Now()
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-perm-id"},
	}

	perm, err := svc.NewPermission(CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "deny-users",
		DisplayName:    "Deny Users",
		Effect:         "deny",
	})
	if err != nil {
		t.Fatalf("NewPermission() error = %v", err)
	}
	if perm.Effect != "deny" {
		t.Errorf("Effect = %q, want %q", perm.Effect, "deny")
	}
}

func TestService_NewPermission_DefaultState(t *testing.T) {
	now := time.Now()
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-perm-id"},
	}

	perm, err := svc.NewPermission(CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "read-users",
		DisplayName:    "Read Users",
		State:          "",
	})
	if err != nil {
		t.Fatalf("NewPermission() error = %v", err)
	}
	if perm.State != "active" {
		t.Errorf("State = %q, want %q", perm.State, "active")
	}
}

func TestService_NewPermission_IsEnabledDefault(t *testing.T) {
	now := time.Now()
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-perm-id"},
	}

	perm, err := svc.NewPermission(CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "read-users",
		DisplayName:    "Read Users",
		IsEnabled:      nil,
	})
	if err != nil {
		t.Fatalf("NewPermission() error = %v", err)
	}
	if !perm.IsEnabled {
		t.Error("IsEnabled = false, want true (default)")
	}
}

func TestService_NewPermission_SliceCopies(t *testing.T) {
	now := time.Now()
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-perm-id"},
	}

	users := []string{"user-1", "user-2"}
	resources := []string{"res-1"}
	actions := []string{"read", "write"}

	perm, err := svc.NewPermission(CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "read-users",
		DisplayName:    "Read Users",
		Users:          users,
		Resources:      resources,
		Actions:        actions,
	})
	if err != nil {
		t.Fatalf("NewPermission() error = %v", err)
	}

	users[0] = "modified"
	if perm.Users[0] == "modified" {
		t.Error("modifying original Users slice affected permission - slices should be copied")
	}
}

func TestService_NewPermission_UUIDError(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &permErrorUUID{},
	}

	_, err := svc.NewPermission(CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "read-users",
		DisplayName:    "Read Users",
	})
	if err == nil {
		t.Fatal("NewPermission() expected error when UUID fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests for permissionEnforcerID
// ---------------------------------------------------------------------------

func TestPermissionEnforcerID_EmptyResources(t *testing.T) {
	perm := &permissiondomain.Permission{Resources: []string{}}
	_, err := permissionEnforcerID(perm)
	if err == nil {
		t.Fatal("permissionEnforcerID() expected error for empty resources, got nil")
	}
}

func TestPermissionEnforcerID_NilResources(t *testing.T) {
	perm := &permissiondomain.Permission{Resources: nil}
	_, err := permissionEnforcerID(perm)
	if err == nil {
		t.Fatal("permissionEnforcerID() expected error for nil resources, got nil")
	}
}

func TestPermissionEnforcerID_BlankEnforcerID(t *testing.T) {
	perm := &permissiondomain.Permission{Resources: []string{"   "}}
	_, err := permissionEnforcerID(perm)
	if err == nil {
		t.Fatal("permissionEnforcerID() expected error for blank enforcer ID, got nil")
	}
}

func TestPermissionEnforcerID_Success(t *testing.T) {
	perm := &permissiondomain.Permission{Resources: []string{"enforcer-1"}}
	id, err := permissionEnforcerID(perm)
	if err != nil {
		t.Fatalf("permissionEnforcerID() unexpected error: %v", err)
	}
	if id != "enforcer-1" {
		t.Errorf("id = %q, want %q", id, "enforcer-1")
	}
}

// ---------------------------------------------------------------------------
// Tests for Service.Enforce with full UOW
// ---------------------------------------------------------------------------

func TestService_Enforce_WithCasbinSuccess(t *testing.T) {
	mgr := &mockCasbinManager{enforceResult: true}
	svc := Service{
		CasbinManager: mgr,
		UOW:           enforceUOW{},
	}

	allowed, err := svc.Enforce(context.Background(), "org-1", "enf-1", EnforceRequest{
		Sub: "alice", Dom: "org1", Obj: "data1", Act: "read",
	})
	if err != nil {
		t.Fatalf("Enforce() error = %v", err)
	}
	if !allowed {
		t.Error("allowed = false, want true")
	}
}

func TestService_Enforce_CasbinError(t *testing.T) {
	mgr := &mockCasbinManager{enforceErr: apperrors.NewInternal("casbin internal error")}
	svc := Service{
		CasbinManager: mgr,
		UOW:           enforceUOW{},
	}

	_, err := svc.Enforce(context.Background(), "org-1", "enf-1", EnforceRequest{
		Sub: "alice", Obj: "data1", Act: "read",
	})
	if err == nil {
		t.Fatal("Enforce() expected error, got nil")
	}
}

func TestService_BatchEnforce_WithCasbinSuccess(t *testing.T) {
	mgr := &mockCasbinManager{batchEnforceResult: []bool{true, false}}
	svc := Service{
		CasbinManager: mgr,
		UOW:           enforceUOW{},
	}

	results, err := svc.BatchEnforce(context.Background(), "org-1", "enf-1", []EnforceRequest{
		{Sub: "alice", Dom: "org1", Obj: "data1", Act: "read"},
		{Sub: "bob", Dom: "org1", Obj: "data1", Act: "write"},
	})
	if err != nil {
		t.Fatalf("BatchEnforce() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if !results[0] {
		t.Error("results[0] = false, want true")
	}
	if results[1] {
		t.Error("results[1] = true, want false")
	}
}

func TestService_GetPolicies_WithEnforcer(t *testing.T) {
	expectedPolicies := [][]string{{"p", "alice", "data1", "read"}}
	mgr := &mockCasbinManager{policiesResult: expectedPolicies}
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Resources: []string{testPolicyEnforcerID}},
		},
	}
	svc := Service{
		CasbinManager: mgr,
		UOW:           enforceUOWWithPerm{permRepo: repo},
	}

	policies, err := svc.GetPolicies(context.Background(), "org-1", "perm-1")
	if err != nil {
		t.Fatalf("GetPolicies() error = %v", err)
	}
	if len(policies) != 1 {
		t.Fatalf("len(policies) = %d, want 1", len(policies))
	}
}

func TestService_GetPolicies_PermissionNotFound(t *testing.T) {
	mgr := &mockCasbinManager{}
	svc := Service{
		CasbinManager: mgr,
		UOW:           enforceUOWWithPerm{permRepo: &mockPermissionRepo{}},
	}

	_, err := svc.GetPolicies(context.Background(), "org-1", "nonexistent")
	if err == nil {
		t.Fatal("GetPolicies() expected error for nonexistent permission, got nil")
	}
}

func TestService_GetPolicies_NoEnforcer(t *testing.T) {
	mgr := &mockCasbinManager{}
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Resources: []string{}},
		},
	}
	svc := Service{
		CasbinManager: mgr,
		UOW:           enforceUOWWithPerm{permRepo: repo},
	}

	_, err := svc.GetPolicies(context.Background(), "org-1", "perm-1")
	if err == nil {
		t.Fatal("GetPolicies() expected error for permission without enforcer, got nil")
	}
}

func TestService_UpdatePolicies_WithEnforcer(t *testing.T) {
	mgr := &mockCasbinManager{}
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Resources: []string{testPolicyEnforcerID}},
		},
	}
	svc := Service{
		CasbinManager: mgr,
		UOW:           enforceUOWWithPerm{permRepo: repo},
	}

	err := svc.UpdatePolicies(context.Background(), "org-1", "perm-1", []PolicyItem{
		{PType: "p", Rules: []string{"alice", "data1", "read"}},
		{PType: "p", Rules: []string{}},
	})
	if err != nil {
		t.Fatalf("UpdatePolicies() error = %v", err)
	}
}

func TestService_UpdatePolicies_PermissionNotFound(t *testing.T) {
	mgr := &mockCasbinManager{}
	svc := Service{
		CasbinManager: mgr,
		UOW:           enforceUOWWithPerm{permRepo: &mockPermissionRepo{}},
	}

	err := svc.UpdatePolicies(context.Background(), "org-1", "nonexistent", nil)
	if err == nil {
		t.Fatal("UpdatePolicies() expected error for nonexistent permission, got nil")
	}
}

func TestService_GetAllObjects_Success(t *testing.T) {
	mgr := &mockCasbinManagerWithObjects{objects: []string{"data1", "data2"}}
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Resources: []string{testPolicyEnforcerID}},
		},
	}
	svc := Service{
		CasbinManager: mgr,
		UOW:           enforceUOWWithPerm{permRepo: repo},
	}

	objects, err := svc.GetAllObjects(context.Background(), "org-1", "perm-1")
	if err != nil {
		t.Fatalf("GetAllObjects() error = %v", err)
	}
	if len(objects) != 2 {
		t.Errorf("len(objects) = %d, want 2", len(objects))
	}
}

func TestService_GetAllActions_Success(t *testing.T) {
	mgr := &mockCasbinManagerWithActions{actions: []string{"read", "write"}}
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Resources: []string{testPolicyEnforcerID}},
		},
	}
	svc := Service{
		CasbinManager: mgr,
		UOW:           enforceUOWWithPerm{permRepo: repo},
	}

	actions, err := svc.GetAllActions(context.Background(), "org-1", "perm-1")
	if err != nil {
		t.Fatalf("GetAllActions() error = %v", err)
	}
	if len(actions) != 2 {
		t.Errorf("len(actions) = %d, want 2", len(actions))
	}
}

// ---------------------------------------------------------------------------
// Helper types
// ---------------------------------------------------------------------------

type permErrorUUID struct{}

func (g *permErrorUUID) NewString() (string, error) { return "", apperrors.NewInternal("uuid failed") }

// enforceUOW provides enforcers and models repos for getModelText
type enforceUOW struct{ testUnitOfWork }

func (u enforceUOW) Outbox() domainoutbox.Repository { return nil }
func (u enforceUOW) IDV() idvdomain.Repository       { return nil }

func (u enforceUOW) Workload() workloaddomain.Repository { return nil }

func (u enforceUOW) Risk() riskdomain.Repository { return nil }

func (u enforceUOW) Actions() actionsdomain.Repository { return nil }

func (u enforceUOW) Metering() meteringdomain.Repository { return nil }

func (u enforceUOW) Enforcers() enforcerdomain.Repository { return &permMockEnforcerRepo{} }
func (u enforceUOW) Models() modeldomain.Repository       { return &permMockModelRepo{} }
func (u enforceUOW) IGA() igadomain.Repository            { return nil }

// enforceUOWWithPerm provides permission + enforcer + model repos
type enforceUOWWithPerm struct {
	testUnitOfWork
	permRepo *mockPermissionRepo
}

func (u enforceUOWWithPerm) Outbox() domainoutbox.Repository          { return nil }
func (u enforceUOWWithPerm) Permissions() permissiondomain.Repository { return u.permRepo }
func (u enforceUOWWithPerm) Enforcers() enforcerdomain.Repository     { return &permMockEnforcerRepo{} }
func (u enforceUOWWithPerm) IDV() idvdomain.Repository                { return nil }

func (u enforceUOWWithPerm) Workload() workloaddomain.Repository { return nil }

func (u enforceUOWWithPerm) Risk() riskdomain.Repository { return nil }

func (u enforceUOWWithPerm) Actions() actionsdomain.Repository { return nil }

func (u enforceUOWWithPerm) Metering() meteringdomain.Repository { return nil }

func (u enforceUOWWithPerm) Models() modeldomain.Repository { return &permMockModelRepo{} }
func (u enforceUOWWithPerm) IGA() igadomain.Repository      { return nil }

type permMockEnforcerRepo struct{}

func (r *permMockEnforcerRepo) GetByID(ctx context.Context, id string) (*enforcerdomain.Enforcer, error) {
	return &enforcerdomain.Enforcer{ID: id, ModelID: "model-1"}, nil
}
func (r *permMockEnforcerRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*enforcerdomain.Enforcer, error) {
	return &enforcerdomain.Enforcer{ID: id, OrganizationID: organizationID, ModelID: "model-1"}, nil
}
func (r *permMockEnforcerRepo) List(ctx context.Context, query enforcerdomain.ListQuery) ([]*enforcerdomain.Enforcer, int64, error) {
	return nil, 0, nil
}
func (r *permMockEnforcerRepo) Create(ctx context.Context, e *enforcerdomain.Enforcer) error {
	return nil
}
func (r *permMockEnforcerRepo) Update(ctx context.Context, e *enforcerdomain.Enforcer) error {
	return nil
}
func (r *permMockEnforcerRepo) Delete(ctx context.Context, id, organizationID string) error {
	return nil
}

type permMockModelRepo struct{}

func (r *permMockModelRepo) GetByID(ctx context.Context, id string) (*modeldomain.Model, error) {
	return &modeldomain.Model{ID: id, ModelText: "[request_definition]\nr = sub, dom, obj, act"}, nil
}
func (r *permMockModelRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*modeldomain.Model, error) {
	return &modeldomain.Model{ID: id, OrganizationID: organizationID, ModelText: "[request_definition]\nr = sub, dom, obj, act"}, nil
}
func (r *permMockModelRepo) List(ctx context.Context, query modeldomain.ListQuery) ([]*modeldomain.Model, int64, error) {
	return nil, 0, nil
}
func (r *permMockModelRepo) Create(ctx context.Context, m *modeldomain.Model) error { return nil }
func (r *permMockModelRepo) Update(ctx context.Context, m *modeldomain.Model) error { return nil }
func (r *permMockModelRepo) Delete(ctx context.Context, id, organizationID string) error {
	return nil
}

// mockCasbinManagerWithObjects extends mockCasbinManager with GetAllObjects
type mockCasbinManagerWithObjects struct {
	mockCasbinManager
	objects []string
}

func (m *mockCasbinManagerWithObjects) GetAllObjects(enforcerID, modelText string) ([]string, error) {
	return m.objects, nil
}

func (m *mockCasbinManagerWithObjects) GetOrgSubjectPolicies(orgID, subject string) ([][]string, error) {
	return nil, nil
}

// mockCasbinManagerWithActions extends mockCasbinManager with GetAllActions
type mockCasbinManagerWithActions struct {
	mockCasbinManager
	actions []string
}

func (m *mockCasbinManagerWithActions) GetAllActions(enforcerID, modelText string) ([]string, error) {
	return m.actions, nil
}

func (m *mockCasbinManagerWithActions) GetOrgSubjectPolicies(orgID, subject string) ([][]string, error) {
	return nil, nil
}

// Ensure error codes work correctly
func TestService_Enforce_ErrorCodes(t *testing.T) {
	svc := Service{}

	_, err := svc.Enforce(context.Background(), "", "", EnforceRequest{})
	if err == nil {
		t.Fatal("expected error for empty enforcer ID")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeInvalidArgument)
	}
}
