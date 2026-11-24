package auth

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	appdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	appmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	captchamock "github.com/guliangiam/guliangiam/internal/domain/captchaproviders/mock"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	providermock "github.com/guliangiam/guliangiam/internal/domain/providers/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// orgAppProviderSvc 构造 Service，为 Organizations / Applications / Providers
// 设置 mock。captchaProviders 返回空（GetCaptchaStatus 降级 → 不过度 mock）。
func orgAppProviderSvc(ctrl *gomock.Controller,
	org *orgdomain.Organization, orgErr error,
	app *appdomain.Application, appExpectCall bool,
	providers []*providerdomain.Provider, provErr error,
) Service {
	uow := mock.NewMockUnitOfWork(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)
	provRepo := providermock.NewMockRepository(ctrl)

	uow.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().GetBySlug(gomock.Any(), "acme").Return(org, orgErr)

	// 只有 org 成功后才会调用 GetCaptchaStatus → CaptchaProviders().List
	// org 失败时不会调用。此 mock 覆盖 org 成功路径。
	if org != nil && orgErr == nil {
		captchaRepo := captchamock.NewMockRepository(ctrl)
		uow.EXPECT().CaptchaProviders().Return(captchaRepo)
		captchaRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
	}

	if appExpectCall {
		uow.EXPECT().Applications().Return(appRepo)
		appRepo.EXPECT().GetByOrganizationAndSlug(gomock.Any(), org.ID, "portal").Return(app, nil)
	}

	if org != nil && orgErr == nil {
		uow.EXPECT().Providers().Return(provRepo)
		provRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(providers, int64(len(providers)), provErr)
	}

	svc := Service{
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		UOW:           uow,
	}
	return svc
}

// ═══════════════════════════════════════════════════════════════════════
// GetAppLogin 测试
// ═══════════════════════════════════════════════════════════════════════

func TestGetAppLogin_OrgNotFound_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := orgAppProviderSvc(ctrl, nil, assert.AnError, nil, false, nil, nil)

	result, err := svc.GetAppLogin(context.Background(), "acme", "")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestGetAppLogin_NoAppSlug_OrgAndProvidersOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{
		ID:          "org-1",
		Slug:        "acme",
		DisplayName: "Acme Inc",
		LogoURL:     "/logo.png",
		FaviconURL:  "/favicon.ico",
		WebsiteURL:  "https://acme.example.com",
	}
	providers := []*providerdomain.Provider{
		{ID: "p1", ProviderType: providerdomain.ProviderGoogle, Category: providerdomain.CategorySocial, DisplayName: "Google", Name: "google"},
		{ID: "p2", ProviderType: providerdomain.ProviderOkta, Category: providerdomain.CategoryEnterprise, DisplayName: "Okta SAML", Name: "okta"},
	}

	svc := orgAppProviderSvc(ctrl, org, nil, nil, false, providers, nil)

	result, err := svc.GetAppLogin(context.Background(), "acme", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Org info
	assert.Equal(t, "org-1", result.Organization.ID)
	assert.Equal(t, "acme", result.Organization.Slug)
	assert.Equal(t, "Acme Inc", result.Organization.DisplayName)
	assert.Equal(t, "/logo.png", result.Organization.LogoURL)

	// No AppLoginApplication when appSlug == ""
	assert.Nil(t, result.Application)

	// Captcha not required (no providers → GetCaptchaStatus returns Required=false)
	assert.Nil(t, result.CaptchaStatus)

	// Providers
	require.Len(t, result.Providers, 2)
	assert.Equal(t, "Google", result.Providers[0].DisplayName)
	assert.Equal(t, "google", result.Providers[0].Name)
	assert.Equal(t, "google", result.Providers[0].Type)
	assert.Equal(t, "Social", result.Providers[0].Category)
}

func TestGetAppLogin_WithAppSlug_FullResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{ID: "org-1", Slug: "acme", DisplayName: "Acme"}
	app := &appdomain.Application{
		ID:            "app-1",
		Slug:          "portal",
		DisplayName:   "Acme Portal",
		Logo:          "/app-logo.png",
		EnableSignup:  true,
		SigninMethods: []string{"password", "sms"},
	}
	providers := []*providerdomain.Provider{}

	svc := orgAppProviderSvc(ctrl, org, nil, app, true, providers, nil)

	result, err := svc.GetAppLogin(context.Background(), "acme", "portal")
	require.NoError(t, err)
	require.NotNil(t, result)

	require.NotNil(t, result.Application)
	assert.Equal(t, "app-1", result.Application.ID)
	assert.Equal(t, "portal", result.Application.Slug)
	assert.Equal(t, "Acme Portal", result.Application.DisplayName)
	assert.Equal(t, "/app-logo.png", result.Application.Logo)
	assert.True(t, result.Application.EnableSignup)
	assert.True(t, result.Application.EnablePassword)
	assert.False(t, result.Application.EnableCodeSignin)
	assert.Equal(t, []string{"password", "sms"}, result.Application.SigninMethods)
}

func TestGetAppLogin_WithAppSlug_AppNotFound_SkipsApp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{ID: "org-1", Slug: "acme", DisplayName: "Acme"}
	// Use custom helper: org OK, GetCaptchaStatus ok, app EXPECTED to return error on any slug
	uow := mock.NewMockUnitOfWork(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)
	captchaRepo := captchamock.NewMockRepository(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)
	provRepo := providermock.NewMockRepository(ctrl)

	uow.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().GetBySlug(gomock.Any(), "acme").Return(org, nil)
	uow.EXPECT().CaptchaProviders().Return(captchaRepo)
	captchaRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
	uow.EXPECT().Applications().Return(appRepo)
	// 期望 GetByOrganizationAndSlug 被调用，返回 error
	appRepo.EXPECT().GetByOrganizationAndSlug(gomock.Any(), "org-1", "unknown").Return(nil, assert.AnError)
	uow.EXPECT().Providers().Return(provRepo)
	provRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)

	svc := Service{
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		UOW:           uow,
	}

	result, err := svc.GetAppLogin(context.Background(), "acme", "unknown")
	require.NoError(t, err) // GetAppLogin itself does not return inner errors
	require.NotNil(t, result)
	assert.Nil(t, result.Application) // Application not populated on repo error
}

func TestGetAppLogin_ProvidersError_SkipsProviders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{ID: "org-1", Slug: "acme", DisplayName: "Acme"}
	svc := orgAppProviderSvc(ctrl, org, nil, nil, false, nil, assert.AnError)

	result, err := svc.GetAppLogin(context.Background(), "acme", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Providers) // Inner if err == nil skips appending
}
