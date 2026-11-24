package permissions

import (
	"context"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	enforcermock "github.com/guliangiam/guliangiam/internal/domain/enforcers/mock"
	modelmock "github.com/guliangiam/guliangiam/internal/domain/models/mock"
	permdomain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	permmock "github.com/guliangiam/guliangiam/internal/domain/permissions/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func expectPermissionTransaction(uowMock *appuowmock.MockUnitOfWork) {
	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uowMock)
		})
}

// OPEN-API-44 ①：CreatePermission 在写库/Casbin 同步前校验 resources[0] 归属；
// 他租户 enforcer 返回 404 且不写 Permission 行。
func TestCreatePermission_RejectsForeignEnforcer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := appuowmock.NewMockUnitOfWork(ctrl)
	enforcerRepo := enforcermock.NewMockRepository(ctrl)
	casbin := &policyCountingCasbin{}
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock, CasbinManager: casbin}

	expectPermissionTransaction(uowMock)
	uowMock.EXPECT().Enforcers().Return(enforcerRepo)
	enforcerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), testPolicyForeignEnforcerID, "org-1").
		Return(nil, apperrors.NewNotFound("enforcer not found"))

	err := svc.CreatePermission(context.Background(), CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "portal-read",
		DisplayName:    "Portal Read",
		ResourceType:   "Application",
		Resources:      []string{testPolicyForeignEnforcerID},
		Effect:         "allow",
		State:          "approved",
	})
	requirePermissionErrorCode(t, err, apperrors.CodeNotFound)
	assert.Zero(t, casbin.reads+casbin.writes, "rejected create must not touch Casbin")
}

func TestCreatePermission_RejectsNonUUIDEnforcerBeforeLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := appuowmock.NewMockUnitOfWork(ctrl)
	casbin := &policyCountingCasbin{}
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock, CasbinManager: casbin}

	expectPermissionTransaction(uowMock)

	err := svc.CreatePermission(context.Background(), CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "portal-read",
		DisplayName:    "Portal Read",
		ResourceType:   "Application",
		Resources:      []string{"not-a-uuid"},
		Effect:         "allow",
		State:          "approved",
	})
	requirePermissionErrorCode(t, err, apperrors.CodeNotFound)
	assert.Zero(t, casbin.reads+casbin.writes, "rejected create must not touch Casbin")
}

// OPEN-API-44 ①：PatchPermission 在写库/重同步前校验变更后的 resources[0]；
// 跨租户返回 404 且不写 Permission 行。
func TestPatchPermission_RejectsForeignEnforcer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := appuowmock.NewMockUnitOfWork(ctrl)
	permRepo := permmock.NewMockRepository(ctrl)
	enforcerRepo := enforcermock.NewMockRepository(ctrl)
	casbin := &policyCountingCasbin{}
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock, CasbinManager: casbin}

	expectPermissionTransaction(uowMock)
	uowMock.EXPECT().Permissions().Return(permRepo)
	permRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "perm-1", "org-1").Return(&permdomain.Permission{
		ID: "perm-1", OrganizationID: "org-1", Resources: []string{testPolicyForeignEnforcerID}, Revision: 1,
	}, nil)
	permRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Times(0)
	uowMock.EXPECT().Enforcers().Return(enforcerRepo)
	enforcerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), testPolicyForeignEnforcerID, "org-1").
		Return(nil, apperrors.NewNotFound("enforcer not found"))

	err := svc.PatchPermission(context.Background(), "org-1", PatchPermissionCommand{ID: "perm-1"})
	requirePermissionErrorCode(t, err, apperrors.CodeNotFound)
	assert.Zero(t, casbin.reads+casbin.writes, "rejected patch must not touch Casbin")
}

func requirePermissionErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, code, appErr.Code)
}

// resources[0] 的 enforcer 属于本租户、但其 model 指向他租户时同样拒绝：
// casbin_rules 共享同一 enforcer/model 文本，模型归属是第二道边界。
func TestCreatePermission_RejectsForeignModel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := appuowmock.NewMockUnitOfWork(ctrl)
	enforcerRepo := enforcermock.NewMockRepository(ctrl)
	modelRepo := modelmock.NewMockRepository(ctrl)
	casbin := &policyCountingCasbin{}
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock, CasbinManager: casbin}

	expectPermissionTransaction(uowMock)
	uowMock.EXPECT().Enforcers().Return(enforcerRepo)
	enforcerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), testPolicyEnforcerID, "org-1").
		Return(&enforcerdomain.Enforcer{ID: testPolicyEnforcerID, OrganizationID: "org-1", ModelID: testPolicyForeignModelID}, nil)
	uowMock.EXPECT().Models().Return(modelRepo)
	modelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), testPolicyForeignModelID, "org-1").
		Return(nil, apperrors.NewNotFound("model not found"))

	err := svc.CreatePermission(context.Background(), CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "portal-read",
		DisplayName:    "Portal Read",
		ResourceType:   "Application",
		Resources:      []string{testPolicyEnforcerID},
		Effect:         "allow",
		State:          "approved",
	})
	requirePermissionErrorCode(t, err, apperrors.CodeNotFound)
}
