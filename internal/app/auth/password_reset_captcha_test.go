package auth

import (
	"context"
	"testing"

	captchaproviders "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	captchamock "github.com/guliangiam/guliangiam/internal/domain/captchaproviders/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ResetPassword must honour the organization's captcha provider. Before this
// gate existed the console rendered a captcha on the reset step but the token
// was dropped, so calling the API directly could change a password with only
// the verification code.
func TestResetPasswordRejectsMissingCaptchaWhenOrganizationEnablesIt(t *testing.T) {
	service, uow := newHistoryStack(0)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := captchamock.NewMockRepository(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*captchaproviders.CaptchaProvider{
		{ProviderType: "default", IsEnabled: true},
	}, int64(1), nil)
	uow.captcha = repo
	service.CaptchaFactory = &fakeCaptchaFactory{verifyResult: true}

	_, _, err := service.ResetPassword(context.Background(), ResetPasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		VerificationCode: "123456",
		NewPassword:      "Reset!A2026",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

func TestResetPasswordAcceptsValidCaptchaWhenOrganizationEnablesIt(t *testing.T) {
	service, uow := newHistoryStack(0)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := captchamock.NewMockRepository(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*captchaproviders.CaptchaProvider{
		{ProviderType: "default", IsEnabled: true},
	}, int64(1), nil)
	uow.captcha = repo
	factory := &fakeCaptchaFactory{verifyResult: true}
	service.CaptchaFactory = factory

	_, _, err := service.ResetPassword(context.Background(), ResetPasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		VerificationCode: "123456",
		NewPassword:      "Reset!A2026",
		CaptchaToken:     "123456",
		CaptchaID:        "cap-1",
		ClientIP:         "1.2.3.4",
	})
	require.NoError(t, err)
	assert.Equal(t, "cap-1:123456", factory.lastVerifyToken)
	assert.Equal(t, "1.2.3.4", factory.lastVerifyRemoteIP)
}
