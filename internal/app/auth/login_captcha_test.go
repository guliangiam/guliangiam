package auth

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/applications"
	captchaproviderdomain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	captchamock "github.com/guliangiam/guliangiam/internal/domain/captchaproviders/mock"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// newCaptchaGateUow wires the login test UOW with an enabled "default" captcha
// provider so EnforceCaptcha resolves a row and the gate becomes fail-close.
func newCaptchaGateUow(
	ctrl *gomock.Controller,
	org *orgdomain.Organization,
	user *userdomain.User,
	app *applications.Application,
) loginTestUnitOfWork {
	uow := newTestUow(org, user, app)
	repo := captchamock.NewMockRepository(ctrl)
	repo.EXPECT().
		List(gomock.Any(), gomock.Any()).
		Return([]*captchaproviderdomain.CaptchaProvider{{ProviderType: "default"}}, int64(1), nil).
		AnyTimes()
	uow.captchaRepo = repo
	return uow
}

func TestLogin_CaptchaEnabled_MissingToken_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uow := newCaptchaGateUow(ctrl, newTestOrg(), newTestUser(), newTestApp())
	svc := newTestService(uow)
	svc.CaptchaFactory = &fakeCaptchaFactory{verifyResult: true}

	// Even correct credentials must be rejected: the gate runs before any
	// credential check, so no session may be created.
	_, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Empty(t, uow.sessRepo.sessions)
}

func TestLogin_CaptchaEnabled_ValidToken_Succeeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uow := newCaptchaGateUow(ctrl, newTestOrg(), newTestUser(), newTestApp())
	svc := newTestService(uow)
	factory := &fakeCaptchaFactory{verifyResult: true}
	svc.CaptchaFactory = factory

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
		CaptchaToken:     "123456",
		CaptchaID:        "cap-1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, LoginStatusSuccess, result.Response.Status)
	// The default provider binds the response to the issued challenge id.
	assert.Equal(t, "cap-1:123456", factory.lastVerifyToken)
}

func TestLoginWithLDAP_CaptchaEnabled_MissingToken_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uow := newCaptchaGateUow(ctrl, newTestOrg(), newTestUser(), newTestApp())
	svc := newTestService(uow)
	svc.CaptchaFactory = &fakeCaptchaFactory{verifyResult: true}

	// The LDAP channel must not bypass the organization captcha policy; the
	// gate fires before any directory bind is attempted.
	_, err := svc.LoginWithLDAP(context.Background(), LdapLoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}
