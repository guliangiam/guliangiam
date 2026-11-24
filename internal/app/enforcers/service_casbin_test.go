package enforcers

import (
	"context"
	"testing"
	"time"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	repoMock "github.com/guliangiam/guliangiam/internal/domain/enforcers/mock"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	modelMock "github.com/guliangiam/guliangiam/internal/domain/models/mock"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// casbinMockForTest implements the CasbinPolicyManager interface for testing
type casbinMockForTest struct {
	mockCasbinManager
	enforceFn        func(enforcerID, modelText string, rvals ...interface{}) (bool, error)
	batchEnforceFn   func(enforcerID, modelText string, rvals [][]interface{}) ([]bool, error)
	addPolicyFn      func(enforcerID, modelText string, params []string) error
	removePolicyFn   func(enforcerID, modelText string, params []string) error
	getPoliciesFn    func(enforcerID, modelText string) ([][]string, error)
	reloadPolicyFn   func(enforcerID, modelText string) error
	getAllSubjectsFn func(enforcerID, modelText string) ([]string, error)
	getAllObjectsFn  func(enforcerID, modelText string) ([]string, error)
	getAllActionsFn  func(enforcerID, modelText string) ([]string, error)
	getAllRolesFn    func(enforcerID, modelText string) ([]string, error)
	updatePolicyFn   func(enforcerID, modelText string, oldPolicy, newPolicy []string) error
}

func (m *casbinMockForTest) Enforce(ctx context.Context, enforcerID, modelText string, rvals ...interface{}) (bool, error) {
	if m.enforceFn != nil {
		return m.enforceFn(enforcerID, modelText, rvals...)
	}
	return m.mockCasbinManager.Enforce(ctx, enforcerID, modelText, rvals...)
}
func (m *casbinMockForTest) BatchEnforce(ctx context.Context, enforcerID, modelText string, rvals [][]interface{}) ([]bool, error) {
	if m.batchEnforceFn != nil {
		return m.batchEnforceFn(enforcerID, modelText, rvals)
	}
	return m.mockCasbinManager.BatchEnforce(ctx, enforcerID, modelText, rvals)
}
func (m *casbinMockForTest) AddPolicy(enforcerID, modelText string, params []string) error {
	if m.addPolicyFn != nil {
		return m.addPolicyFn(enforcerID, modelText, params)
	}
	return m.mockCasbinManager.AddPolicy(enforcerID, modelText, params)
}
func (m *casbinMockForTest) RemovePolicy(enforcerID, modelText string, params []string) error {
	if m.removePolicyFn != nil {
		return m.removePolicyFn(enforcerID, modelText, params)
	}
	return m.mockCasbinManager.RemovePolicy(enforcerID, modelText, params)
}
func (m *casbinMockForTest) GetPolicies(enforcerID, modelText string) ([][]string, error) {
	if m.getPoliciesFn != nil {
		return m.getPoliciesFn(enforcerID, modelText)
	}
	return m.mockCasbinManager.GetPolicies(enforcerID, modelText)
}
func (m *casbinMockForTest) ReloadPolicy(enforcerID, modelText string) error {
	if m.reloadPolicyFn != nil {
		return m.reloadPolicyFn(enforcerID, modelText)
	}
	return m.mockCasbinManager.ReloadPolicy(enforcerID, modelText)
}
func (m *casbinMockForTest) GetAllSubjects(enforcerID, modelText string) ([]string, error) {
	if m.getAllSubjectsFn != nil {
		return m.getAllSubjectsFn(enforcerID, modelText)
	}
	return m.mockCasbinManager.GetAllSubjects(enforcerID, modelText)
}
func (m *casbinMockForTest) GetAllObjects(enforcerID, modelText string) ([]string, error) {
	if m.getAllObjectsFn != nil {
		return m.getAllObjectsFn(enforcerID, modelText)
	}
	return m.mockCasbinManager.GetAllObjects(enforcerID, modelText)
}
func (m *casbinMockForTest) GetAllActions(enforcerID, modelText string) ([]string, error) {
	if m.getAllActionsFn != nil {
		return m.getAllActionsFn(enforcerID, modelText)
	}
	return m.mockCasbinManager.GetAllActions(enforcerID, modelText)
}
func (m *casbinMockForTest) GetAllRoles(enforcerID, modelText string) ([]string, error) {
	if m.getAllRolesFn != nil {
		return m.getAllRolesFn(enforcerID, modelText)
	}
	return m.mockCasbinManager.GetAllRoles(enforcerID, modelText)
}
func (m *casbinMockForTest) UpdatePolicy(enforcerID, modelText string, oldPolicy, newPolicy []string) error {
	if m.updatePolicyFn != nil {
		return m.updatePolicyFn(enforcerID, modelText, oldPolicy, newPolicy)
	}
	return m.mockCasbinManager.UpdatePolicy(enforcerID, modelText, oldPolicy, newPolicy)
}

func (m *casbinMockForTest) GetOrgSubjectPolicies(orgID, subject string) ([][]string, error) {
	return nil, nil
}

func TestService_Enforce_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	mockModelRepo := modelMock.NewMockRepository(ctrl)
	casbin := &casbinMockForTest{
		enforceFn: func(enforcerID, modelText string, rvals ...interface{}) (bool, error) {
			return true, nil
		},
	}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockUOW.EXPECT().Models().Return(mockModelRepo).AnyTimes()
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", IsEnabled: true, ModelID: "model-1"}, nil)
	mockModelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "model-1", "org-1").Return(&modeldomain.Model{ID: "model-1", ModelText: "model_text"}, nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	result, err := svc.Enforce(context.Background(), "org-1", EnforceCommand{ID: "enf-1", V0: "alice", V1: "data1", V2: "read"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Allow)
}

func TestService_Enforce_DisabledEnforcer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	casbin := &casbinMockForTest{}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo)
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", IsEnabled: false}, nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	_, err := svc.Enforce(context.Background(), "org-1", EnforceCommand{ID: "enf-1", V0: "alice", V1: "data1", V2: "read"})
	assert.Error(t, err)
}

func TestService_Enforce_NoModel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	casbin := &casbinMockForTest{}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo)
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", IsEnabled: true, ModelID: ""}, nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	_, err := svc.Enforce(context.Background(), "org-1", EnforceCommand{ID: "enf-1", V0: "alice", V1: "data1", V2: "read"})
	assert.Error(t, err)
}

func TestService_BatchEnforce_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	mockModelRepo := modelMock.NewMockRepository(ctrl)
	casbin := &casbinMockForTest{
		batchEnforceFn: func(enforcerID, modelText string, rvals [][]interface{}) ([]bool, error) {
			return []bool{true, false}, nil
		},
	}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockUOW.EXPECT().Models().Return(mockModelRepo).AnyTimes()
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", IsEnabled: true, ModelID: "model-1"}, nil)
	mockModelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "model-1", "org-1").Return(&modeldomain.Model{ID: "model-1", ModelText: "model_text"}, nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	result, err := svc.BatchEnforce(context.Background(), "org-1", BatchEnforceCommand{
		ID: "enf-1",
		Requests: []EnforceCommand{
			{V0: "alice", V1: "data1", V2: "read"},
			{V0: "bob", V1: "data2", V2: "write"},
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Results, 2)
}

func TestService_AddPolicy_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	mockModelRepo := modelMock.NewMockRepository(ctrl)
	called := false
	casbin := &casbinMockForTest{
		addPolicyFn: func(enforcerID, modelText string, params []string) error {
			called = true
			return nil
		},
	}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockUOW.EXPECT().Models().Return(mockModelRepo).AnyTimes()
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", ModelID: "model-1"}, nil)
	mockModelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "model-1", "org-1").Return(&modeldomain.Model{ID: "model-1", ModelText: "model_text"}, nil)
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	err := svc.AddPolicy(context.Background(), "org-1", "enf-1", []string{"alice", "data1", "read"})
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestService_RemovePolicy_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	mockModelRepo := modelMock.NewMockRepository(ctrl)
	called := false
	casbin := &casbinMockForTest{
		removePolicyFn: func(enforcerID, modelText string, params []string) error {
			called = true
			return nil
		},
	}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockUOW.EXPECT().Models().Return(mockModelRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).Return(nil)
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", ModelID: "model-1"}, nil)
	mockModelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "model-1", "org-1").Return(&modeldomain.Model{ID: "model-1", ModelText: "model_text"}, nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	err := svc.RemovePolicy(context.Background(), "org-1", "enf-1", []string{"alice", "data1", "read"})
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestService_GetPolicies_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	mockModelRepo := modelMock.NewMockRepository(ctrl)
	casbin := &casbinMockForTest{
		getPoliciesFn: func(enforcerID, modelText string) ([][]string, error) {
			return [][]string{{"alice", "data1", "read"}}, nil
		},
	}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockUOW.EXPECT().Models().Return(mockModelRepo).AnyTimes()
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", ModelID: "model-1"}, nil)
	mockModelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "model-1", "org-1").Return(&modeldomain.Model{ID: "model-1", ModelText: "model_text"}, nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	result, err := svc.GetPolicies(context.Background(), "org-1", "enf-1")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestService_ExportPolicies_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	mockModelRepo := modelMock.NewMockRepository(ctrl)
	casbin := &casbinMockForTest{
		getPoliciesFn: func(enforcerID, modelText string) ([][]string, error) {
			return [][]string{{"alice", "data1", "read"}}, nil
		},
	}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockUOW.EXPECT().Models().Return(mockModelRepo).AnyTimes()
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", Name: "test-enforcer", ModelID: "model-1"}, nil)
	mockModelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "model-1", "org-1").Return(&modeldomain.Model{ID: "model-1", ModelText: "model_text"}, nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	result, err := svc.ExportPolicies(context.Background(), "org-1", "enf-1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "enf-1", result.EnforcerID)
	assert.Equal(t, "test-enforcer", result.EnforcerName)
}

func TestService_ImportPolicies_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	mockModelRepo := modelMock.NewMockRepository(ctrl)
	var removedPolicies, addedPolicies [][]string
	casbin := &casbinMockForTest{
		getPoliciesFn: func(enforcerID, modelText string) ([][]string, error) {
			return [][]string{{"old", "policy"}}, nil
		},
		removePolicyFn: func(enforcerID, modelText string, params []string) error {
			removedPolicies = append(removedPolicies, params)
			return nil
		},
		addPolicyFn: func(enforcerID, modelText string, params []string) error {
			addedPolicies = append(addedPolicies, params)
			return nil
		},
	}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockUOW.EXPECT().Models().Return(mockModelRepo).AnyTimes()
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", IsEnabled: true, ModelID: "model-1"}, nil)
	mockModelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "model-1", "org-1").Return(&modeldomain.Model{ID: "model-1", ModelText: "model_text"}, nil)
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	err := svc.ImportPolicies(context.Background(), "org-1", "enf-1", [][]string{{"p", "new", "policy"}})
	assert.NoError(t, err)
	assert.Len(t, removedPolicies, 1)
	assert.Len(t, addedPolicies, 1)
}

func TestService_ImportPolicies_DisabledEnforcer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	casbin := &casbinMockForTest{}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", IsEnabled: false, ModelID: "model-1"}, nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	err := svc.ImportPolicies(context.Background(), "org-1", "enf-1", [][]string{{"p", "new", "policy"}})
	assert.Error(t, err)
}

func TestService_ReloadPolicy_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	mockModelRepo := modelMock.NewMockRepository(ctrl)
	called := false
	casbin := &casbinMockForTest{
		reloadPolicyFn: func(enforcerID, modelText string) error {
			called = true
			return nil
		},
	}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockUOW.EXPECT().Models().Return(mockModelRepo).AnyTimes()
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", ModelID: "model-1"}, nil)
	mockModelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "model-1", "org-1").Return(&modeldomain.Model{ID: "model-1", ModelText: "model_text"}, nil)
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	err := svc.ReloadPolicy(context.Background(), "org-1", "enf-1")
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestService_GetAllSubjects_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	mockModelRepo := modelMock.NewMockRepository(ctrl)
	casbin := &casbinMockForTest{
		getAllSubjectsFn: func(enforcerID, modelText string) ([]string, error) {
			return []string{"alice", "bob"}, nil
		},
	}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockUOW.EXPECT().Models().Return(mockModelRepo).AnyTimes()
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", ModelID: "model-1"}, nil)
	mockModelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "model-1", "org-1").Return(&modeldomain.Model{ID: "model-1", ModelText: "model_text"}, nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	result, err := svc.GetAllSubjects(context.Background(), "org-1", "enf-1")
	assert.NoError(t, err)
	assert.Equal(t, []string{"alice", "bob"}, result)
}

func TestService_UpdatePolicy_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockEnfRepo := repoMock.NewMockRepository(ctrl)
	mockModelRepo := modelMock.NewMockRepository(ctrl)
	called := false
	casbin := &casbinMockForTest{
		updatePolicyFn: func(enforcerID, modelText string, oldPolicy, newPolicy []string) error {
			called = true
			return nil
		},
	}

	mockUOW.EXPECT().Enforcers().Return(mockEnfRepo).AnyTimes()
	mockUOW.EXPECT().Models().Return(mockModelRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).Return(nil)
	mockEnfRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "enf-1", "org-1").Return(&domain.Enforcer{ID: "enf-1", OrganizationID: "org-1", ModelID: "model-1"}, nil)
	mockModelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "model-1", "org-1").Return(&modeldomain.Model{ID: "model-1", ModelText: "model_text"}, nil)

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Now()},
		UUID:          &fakeUUIDGenerator{fixedID: "test"},
		UOW:           mockUOW,
		CasbinManager: casbin,
	}

	err := svc.UpdatePolicy(context.Background(), "org-1", "enf-1", []string{"old"}, []string{"new"})
	assert.NoError(t, err)
	assert.True(t, called)
}
