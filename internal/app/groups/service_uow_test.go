package groups

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/groups"
	groupsmock "github.com/guliangiam/guliangiam/internal/domain/groups/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewService_groups(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	svc := NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uowMock)
	assert.NotNil(t, svc)
	assert.Equal(t, uowMock, svc.UOW)
}

func TestListGroups_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	repo := groupsmock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().Groups().Return(repo)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*groups.Group{}, int64(0), nil)

	items, total, err := svc.ListGroups(context.Background(), ListGroupsQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestListGroups_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	repo := groupsmock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().Groups().Return(repo)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	_, _, err := svc.ListGroups(context.Background(), ListGroupsQuery{Page: 1, PageSize: 10})
	assert.Error(t, err)
}

func TestListGroups_PassesSortAndStatusToRepository(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	repo := groupsmock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().Groups().Return(repo)
	repo.EXPECT().List(gomock.Any(), groups.ListQuery{
		Page:           2,
		PageSize:       20,
		OrganizationID: "org-001",
		Keyword:        "dev",
		Status:         "inactive",
		SortField:      "displayName",
		SortOrder:      "desc",
	}).Return([]*groups.Group{}, int64(0), nil)

	_, _, err := svc.ListGroups(context.Background(), ListGroupsQuery{
		Page:           2,
		PageSize:       20,
		OrganizationID: "org-001",
		Keyword:        "dev",
		Status:         "inactive",
		SortField:      "displayName",
		SortOrder:      "desc",
	})
	require.NoError(t, err)
}
