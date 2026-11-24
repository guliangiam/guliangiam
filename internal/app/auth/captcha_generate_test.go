package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	captchaproviders "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	captchamock "github.com/guliangiam/guliangiam/internal/domain/captchaproviders/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ─── fakeCaptchaFactory 实现 CaptchaFactory ─────────────────────────────

type fakeCaptchaFactory struct {
	generateID         string
	generateImage      string
	generateSubImage   string
	generateShape      string
	generateErr        error
	verifyResult       bool
	verifyErr          error
	lastProviderType   string
	lastGenerateShape  string
	lastVerifyToken    string
	lastVerifyRemoteIP string
	lastRowSiteKey     string
	lastRowSecretKey   string
	lastRowConfig      map[string]any
	rowVerifyCalled    bool
}

func (f *fakeCaptchaFactory) GenerateCaptcha(_ context.Context, providerType, shape string) (captchaID, imageBase64, subImageBase64, resolvedShape string, err error) {
	f.lastProviderType = providerType
	f.lastGenerateShape = shape
	return f.generateID, f.generateImage, f.generateSubImage, f.generateShape, f.generateErr
}

func (f *fakeCaptchaFactory) VerifyCaptcha(_ context.Context, providerType, token, remoteIP string) (bool, error) {
	f.lastProviderType = providerType
	f.lastVerifyToken = token
	f.lastVerifyRemoteIP = remoteIP
	return f.verifyResult, f.verifyErr
}

func (f *fakeCaptchaFactory) VerifyCaptchaWithRow(_ context.Context, providerType, siteKey, secretKey, token, remoteIP string, config map[string]any) (bool, error) {
	f.rowVerifyCalled = true
	f.lastRowSiteKey = siteKey
	f.lastRowSecretKey = secretKey
	f.lastRowConfig = config
	return f.VerifyCaptcha(context.Background(), providerType, token, remoteIP)
}

// newCaptchaTestSvc 构造带 mock UOW + fake CaptchaFactory 的 Service
func newCaptchaTestSvc(ctrl *gomock.Controller, orgID string, providers []*captchaproviders.CaptchaProvider, factory CaptchaFactory) Service {
	uowMock := mock.NewMockUnitOfWork(ctrl)
	if orgID != "" {
		repo := captchamock.NewMockRepository(ctrl)
		uowMock.EXPECT().CaptchaProviders().Return(repo)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(providers, int64(len(providers)), nil)
	}
	return Service{
		Clock:          clock.SystemClock{},
		UUIDGenerator:  platformuuid.V7Generator{},
		UOW:            uowMock,
		CaptchaFactory: factory,
	}
}

// ═══════════════════════════════════════════════════════════════════════
// GenerateCaptcha 测试
// ═══════════════════════════════════════════════════════════════════════

func TestGenerateCaptcha_FactoryNil_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newCaptchaTestSvc(ctrl, "", nil, nil)

	result, err := svc.GenerateCaptcha(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestGenerateCaptcha_EmptyOrgID_DefaultProviderType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{generateID: "cap-1", generateImage: "base64img"}
	svc := newCaptchaTestSvc(ctrl, "", nil, factory)

	result, err := svc.GenerateCaptcha(context.Background(), "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "cap-1", result.CaptchaID)
	assert.Equal(t, "base64img", result.CaptchaImage)
	assert.Equal(t, "default", result.Type)
	assert.Equal(t, "default", factory.lastProviderType)
}

func TestGenerateCaptcha_WithProvider_UsesCorrectType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{generateID: "cap-2", generateImage: "pngbase64"}
	providers := []*captchaproviders.CaptchaProvider{{ProviderType: "recaptcha"}}
	svc := newCaptchaTestSvc(ctrl, "org-1", providers, factory)

	result, err := svc.GenerateCaptcha(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "recaptcha", result.Type)
	assert.Equal(t, "recaptcha", factory.lastProviderType)
}

func TestGenerateCaptcha_FactoryError_WrapsInternal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{generateErr: errors.New("provider down")}
	svc := newCaptchaTestSvc(ctrl, "", nil, factory)

	result, err := svc.GenerateCaptcha(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestGenerateCaptcha_GoCaptcha_PassesRowShapeAndSubImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{
		generateID: "gc-1", generateImage: "masterb64",
		generateSubImage: "subb64", generateShape: "slide",
	}
	providers := []*captchaproviders.CaptchaProvider{
		{ProviderType: "go_captcha", Config: map[string]any{"shape": "slide"}},
	}
	svc := newCaptchaTestSvc(ctrl, "org-1", providers, factory)

	result, err := svc.GenerateCaptcha(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slide", factory.lastGenerateShape)
	assert.Equal(t, "gc-1", result.CaptchaID)
	assert.Equal(t, "go_captcha", result.Type)
	assert.Equal(t, "slide", result.Shape)
	assert.Equal(t, "subb64", result.CaptchaImageSub)
}

func TestGenerateCaptcha_DefaultType_NoShapeFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{generateID: "cap-3", generateImage: "pngb64"}
	svc := newCaptchaTestSvc(ctrl, "", nil, factory)

	result, err := svc.GenerateCaptcha(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, result.Shape)
	assert.Empty(t, result.CaptchaImageSub)
	assert.Empty(t, factory.lastGenerateShape)
}

// ═══════════════════════════════════════════════════════════════════════
// VerifyCaptcha 测试
// ═══════════════════════════════════════════════════════════════════════

func TestVerifyCaptcha_FactoryNil_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newCaptchaTestSvc(ctrl, "", nil, nil)

	result, err := svc.VerifyCaptcha(context.Background(), "", "token-abc", "1.2.3.4")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestVerifyCaptcha_ValidToken_ReturnsTrue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{verifyResult: true}
	svc := newCaptchaTestSvc(ctrl, "", nil, factory)

	result, err := svc.VerifyCaptcha(context.Background(), "", "token-ok", "10.0.0.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, "token-ok", factory.lastVerifyToken)
	assert.Equal(t, "10.0.0.1", factory.lastVerifyRemoteIP)
	assert.Equal(t, "default", factory.lastProviderType)
}

func TestVerifyCaptcha_InvalidToken_ReturnsFalse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{verifyResult: false}
	svc := newCaptchaTestSvc(ctrl, "", nil, factory)

	result, err := svc.VerifyCaptcha(context.Background(), "", "token-bad", "192.168.1.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Valid)
}

func TestVerifyCaptcha_WithProvider_UsesCorrectType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{verifyResult: true}
	providers := []*captchaproviders.CaptchaProvider{{ProviderType: "hcaptcha"}}
	svc := newCaptchaTestSvc(ctrl, "org-99", providers, factory)

	result, err := svc.VerifyCaptcha(context.Background(), "org-99", "tok", "0.0.0.0")
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, "hcaptcha", factory.lastProviderType)
	assert.True(t, factory.rowVerifyCalled, "third-party rows must verify through the row-backed path")
}

func TestVerifyCaptcha_RowCredentials_PreferredOverEnvFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{verifyResult: true}
	providers := []*captchaproviders.CaptchaProvider{{
		ProviderType: "aliyun_captcha",
		SiteKey:      "ak-1",
		SecretKey:    "sk-1",
		Config:       map[string]any{"sceneId": "scene-1"},
	}}
	svc := newCaptchaTestSvc(ctrl, "org-7", providers, factory)

	result, err := svc.VerifyCaptcha(context.Background(), "org-7", "tok", "9.9.9.9")
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.True(t, factory.rowVerifyCalled)
	assert.Equal(t, "ak-1", factory.lastRowSiteKey)
	assert.Equal(t, "sk-1", factory.lastRowSecretKey)
	assert.Equal(t, map[string]any{"sceneId": "scene-1"}, factory.lastRowConfig)
}

func TestVerifyCaptcha_DefaultRow_UsesEnvFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{verifyResult: true}
	providers := []*captchaproviders.CaptchaProvider{{ProviderType: "default", SiteKey: "x", SecretKey: "y"}}
	svc := newCaptchaTestSvc(ctrl, "org-8", providers, factory)

	result, err := svc.VerifyCaptcha(context.Background(), "org-8", "tok", "1.1.1.1")
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.False(t, factory.rowVerifyCalled, "the image captcha store lives on the factory, not the row")
	assert.Equal(t, "default", factory.lastProviderType)
}

func TestVerifyCaptcha_FactoryError_WrapsInternal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	factory := &fakeCaptchaFactory{verifyErr: errors.New("network timeout")}
	svc := newCaptchaTestSvc(ctrl, "", nil, factory)

	result, err := svc.VerifyCaptcha(context.Background(), "", "tok", "ip")
	require.Error(t, err)
	assert.Nil(t, result)
}
