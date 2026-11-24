package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	verifmock "github.com/guliangiam/guliangiam/internal/domain/verifications/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 自助资料更新/绑定（PATCH /api/v1/auth/account）：会话主体只能改自己，绑定联系方式
// 必须先验证发给该联系方式的验证码。

func TestUpdateAccount_BindsVerifiedContact(t *testing.T) {
	ctrl := gomock.NewController(t)
	user := newTestUser()
	users := newQRTestUserRepository(user)
	verifRepo := verifmock.NewMockRepository(ctrl)
	verifRepo.EXPECT().
		GetActiveByRecipientAndCode(gomock.Any(), "13800000000", "123456", "phone").
		Return(&verificationsdomain.Verification{ID: "verification-1", OrganizationID: testOrgID}, nil)
	verifRepo.EXPECT().UpdateStatus(gomock.Any(), "verification-1", "consumed").Return(nil)

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:   newQRTestOrgRepository(newTestOrg()),
		userRepo:  users,
		verifRepo: verifRepo,
	})

	phone := "13800000000"
	displayName := "Guest Renamed"
	account, err := svc.UpdateAccount(context.Background(), UpdateAccountCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Phone:            &phone,
		DisplayName:      &displayName,
		VerificationCode: "123456",
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, phone, users.users[user.ID].Profile.Phone)
	assert.Equal(t, displayName, users.users[user.ID].Profile.DisplayName)
}

func TestUpdateAccount_RequiresVerificationCodeForContactBinding(t *testing.T) {
	ctrl := gomock.NewController(t)
	user := newTestUser()
	users := newQRTestUserRepository(user)
	// 验证码缺失时不得触碰验证码仓储，也不得写入资料。
	verifRepo := verifmock.NewMockRepository(ctrl)

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:   newQRTestOrgRepository(newTestOrg()),
		userRepo:  users,
		verifRepo: verifRepo,
	})

	email := "guest@example.com"
	_, err := svc.UpdateAccount(context.Background(), UpdateAccountCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Email:            &email,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Empty(t, users.users[user.ID].Profile.Email == email)
}

func TestUpdateAccount_RejectsAnonymousCaller(t *testing.T) {
	ctrl := gomock.NewController(t)
	verifRepo := verifmock.NewMockRepository(ctrl)
	svc := newTestService(loginTestUnitOfWork{verifRepo: verifRepo})

	displayName := "Nobody"
	_, err := svc.UpdateAccount(context.Background(), UpdateAccountCommand{DisplayName: &displayName})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
}

func TestUpdateAccount_RejectsEmptyPatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	verifRepo := verifmock.NewMockRepository(ctrl)
	svc := newTestService(loginTestUnitOfWork{verifRepo: verifRepo})

	_, err := svc.UpdateAccount(context.Background(), UpdateAccountCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}
