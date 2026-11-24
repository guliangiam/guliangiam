package auth

import (
	"context"
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

func TestGetCaptchaStatus_EmptyOrgID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	result, err := svc.GetCaptchaStatus(context.Background(), "")
	require.NoError(t, err)
	assert.False(t, result.Required)
}

func TestGetCaptchaStatus_ProvidersFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	captchaRepo := captchamock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	providers := []*captchaproviders.CaptchaProvider{
		{ProviderType: "recaptcha", SiteKey: "site-key-123"},
	}
	uowMock.EXPECT().CaptchaProviders().Return(captchaRepo)
	captchaRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(providers, int64(1), nil)

	result, err := svc.GetCaptchaStatus(context.Background(), "org-1")
	require.NoError(t, err)
	assert.True(t, result.Required)
	assert.Equal(t, "recaptcha", result.ProviderType)
	assert.Equal(t, "site-key-123", result.SiteKey)
}

func TestGetCaptchaStatus_NoProviders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	captchaRepo := captchamock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().CaptchaProviders().Return(captchaRepo)
	captchaRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*captchaproviders.CaptchaProvider{}, int64(0), nil)

	result, err := svc.GetCaptchaStatus(context.Background(), "org-1")
	require.NoError(t, err)
	assert.False(t, result.Required)
}

func TestGetCaptchaStatus_Error_FailOpen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	captchaRepo := captchamock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().CaptchaProviders().Return(captchaRepo)
	captchaRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	result, err := svc.GetCaptchaStatus(context.Background(), "org-1")
	require.NoError(t, err) // Fail-open: no error returned
	assert.False(t, result.Required)
}

func TestResolveCaptchaProvider_EmptyOrgID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	result, row := svc.resolveCaptchaProvider(context.Background(), "")
	assert.Equal(t, "default", result)
	assert.Nil(t, row)
}

func TestResolveCaptchaProvider_ProvidersFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	captchaRepo := captchamock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	providers := []*captchaproviders.CaptchaProvider{
		{ProviderType: "hcaptcha"},
	}
	uowMock.EXPECT().CaptchaProviders().Return(captchaRepo)
	captchaRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(providers, int64(1), nil)

	result, row := svc.resolveCaptchaProvider(context.Background(), "org-1")
	assert.Equal(t, "hcaptcha", result)
	require.NotNil(t, row)
}

func TestResolveCaptchaProvider_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	captchaRepo := captchamock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().CaptchaProviders().Return(captchaRepo)
	captchaRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	result, row := svc.resolveCaptchaProvider(context.Background(), "org-1")
	assert.Equal(t, "default", result)
	assert.Nil(t, row)
}
