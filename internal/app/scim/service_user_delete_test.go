package scim

import (
	"context"
	"testing"

	sessionsmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	tokensmock "github.com/guliangiam/guliangiam/internal/domain/tokens/mock"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// SCIM DELETE = 停用：状态置 is_forbidden、revision++、同事务吊销会话/令牌，
// 而不是物理删除或只回显。审计由平台审计提供者在同一事务写入。
func TestService_DeleteUser_DeactivatesAndRevokesCredentials(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	expectScimTransaction(uowMock)
	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo).Times(2)
	user := &domainusers.User{ID: "user-1", OrganizationID: "org-1", Revision: 2}
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(user, nil)
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), "org-1").DoAndReturn(
		func(ctx context.Context, updated *domainusers.User, orgID string) error {
			assert.True(t, updated.IsForbidden, "SCIM DELETE must set is_forbidden")
			assert.Equal(t, int64(3), updated.Revision)
			assert.Equal(t, scimTestNow, updated.UpdatedAt)
			return nil
		})

	sessionsRepo := sessionsmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Sessions().Return(sessionsRepo)
	sessionsRepo.EXPECT().RevokeByUserExcept(gomock.Any(), "user-1", "", scimTestNow).Return(int64(2), nil)

	tokensRepo := tokensmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Tokens().Return(tokensRepo)
	tokensRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), scimTestNow).Return(int64(3), nil)

	require.NoError(t, svc.DeleteUser(context.Background(), "org-1", "user-1"))
}
