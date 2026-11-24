package permissions

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/permissions"
	permissionsmock "github.com/guliangiam/guliangiam/internal/domain/permissions/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewService_permissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	svc := NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uowMock, nil)
	assert.NotNil(t, svc)
	assert.Equal(t, uowMock, svc.UOW)
}

func TestListPermissions_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	repo := permissionsmock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().Permissions().Return(repo)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*permissions.Permission{}, int64(0), nil)

	items, total, err := svc.ListPermissions(context.Background(), ListPermissionsQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestListPermissions_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	repo := permissionsmock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().Permissions().Return(repo)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	_, _, err := svc.ListPermissions(context.Background(), ListPermissionsQuery{Page: 1, PageSize: 10})
	assert.Error(t, err)
}
