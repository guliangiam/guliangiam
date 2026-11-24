package captchaproviders

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	captchaprovidersmock "github.com/guliangiam/guliangiam/internal/domain/captchaproviders/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewService_captchaproviders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	svc := NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uowMock)
	assert.NotNil(t, svc)
	assert.Equal(t, uowMock, svc.UOW)
}

func TestListCaptchaProviders_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	repo := captchaprovidersmock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().CaptchaProviders().Return(repo)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*captchaproviders.CaptchaProvider{}, int64(0), nil)

	items, total, err := svc.ListCaptchaProviders(context.Background(), ListCaptchaProvidersQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestListCaptchaProviders_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	repo := captchaprovidersmock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().CaptchaProviders().Return(repo)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	_, _, err := svc.ListCaptchaProviders(context.Background(), ListCaptchaProvidersQuery{Page: 1, PageSize: 10})
	assert.Error(t, err)
}
