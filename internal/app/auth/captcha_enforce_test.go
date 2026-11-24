package auth

import (
	"context"
	"testing"

	captchaproviders "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEnforceCaptcha_NoProvider_FailOpen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	// Empty organization resolves to "default"/nil without a repository lookup.
	svc := newCaptchaTestSvc(ctrl, "", nil, &fakeCaptchaFactory{})
	err := svc.EnforceCaptcha(context.Background(), "", "", "", "1.2.3.4")
	assert.NoError(t, err)
}

func TestEnforceCaptcha_NoEnabledRow_FailOpen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newCaptchaTestSvc(ctrl, "org-1", nil, &fakeCaptchaFactory{})
	err := svc.EnforceCaptcha(context.Background(), "org-1", "", "", "1.2.3.4")
	assert.NoError(t, err)
}

func TestEnforceCaptcha_ConfiguredDefault_EmptyToken_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	providers := []*captchaproviders.CaptchaProvider{{ProviderType: "default"}}
	svc := newCaptchaTestSvc(ctrl, "org-1", providers, &fakeCaptchaFactory{verifyResult: true})
	err := svc.EnforceCaptcha(context.Background(), "org-1", "", "cap-1", "1.2.3.4")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

func TestEnforceCaptcha_Default_ReconstructsIDValueToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	providers := []*captchaproviders.CaptchaProvider{{ProviderType: "default"}}
	factory := &fakeCaptchaFactory{verifyResult: true}
	svc := newCaptchaTestSvc(ctrl, "org-1", providers, factory)
	err := svc.EnforceCaptcha(context.Background(), "org-1", "123456", "cap-1", "1.2.3.4")
	require.NoError(t, err)
	assert.Equal(t, "cap-1:123456", factory.lastVerifyToken)
	assert.False(t, factory.rowVerifyCalled)
}

func TestEnforceCaptcha_Default_MissingID_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	providers := []*captchaproviders.CaptchaProvider{{ProviderType: "default"}}
	svc := newCaptchaTestSvc(ctrl, "org-1", providers, &fakeCaptchaFactory{verifyResult: true})
	err := svc.EnforceCaptcha(context.Background(), "org-1", "123456", "", "1.2.3.4")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

func TestEnforceCaptcha_InvalidResponse_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	providers := []*captchaproviders.CaptchaProvider{{ProviderType: "default"}}
	svc := newCaptchaTestSvc(ctrl, "org-1", providers, &fakeCaptchaFactory{verifyResult: false})
	err := svc.EnforceCaptcha(context.Background(), "org-1", "wrong", "cap-1", "1.2.3.4")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

func TestEnforceCaptcha_ThirdParty_UsesRowCredentialsAndOpaqueToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	providers := []*captchaproviders.CaptchaProvider{{
		ProviderType: "recaptcha",
		SiteKey:      "site",
		SecretKey:    "secret",
	}}
	factory := &fakeCaptchaFactory{verifyResult: true}
	svc := newCaptchaTestSvc(ctrl, "org-1", providers, factory)
	err := svc.EnforceCaptcha(context.Background(), "org-1", "widget-token", "ignored-id", "9.9.9.9")
	require.NoError(t, err)
	assert.True(t, factory.rowVerifyCalled)
	assert.Equal(t, "widget-token", factory.lastVerifyToken)
	assert.Equal(t, "secret", factory.lastRowSecretKey)
}

func TestEnforceCaptcha_GoCaptcha_ReconstructsIDValueToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	providers := []*captchaproviders.CaptchaProvider{{ProviderType: "go_captcha"}}
	factory := &fakeCaptchaFactory{verifyResult: true}
	svc := newCaptchaTestSvc(ctrl, "org-1", providers, factory)
	err := svc.EnforceCaptcha(context.Background(), "org-1", "payload", "gc-1", "1.2.3.4")
	require.NoError(t, err)
	assert.Equal(t, "gc-1:payload", factory.lastVerifyToken)
}

func TestEnforceCaptcha_FactoryNil_FailOpen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	// Unwired captcha subsystem (no factory) fails open before any lookup.
	svc := newCaptchaTestSvc(ctrl, "", nil, nil)
	err := svc.EnforceCaptcha(context.Background(), "org-1", "123456", "cap-1", "1.2.3.4")
	assert.NoError(t, err)
}
