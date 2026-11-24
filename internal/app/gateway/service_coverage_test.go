package gateway

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	gwmock "github.com/guliangiam/guliangiam/internal/domain/gateway/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestService_GetServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo)
	repo.EXPECT().GetServerByIDForOrganization(gomock.Any(), "server-001", "org-1").Return(&domain.Server{
		ID: "server-001", OrganizationID: "org-1", Name: "api-server",
		Endpoint: "https://api.example.com", Protocol: "https",
		Status: "active", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}, nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	result, err := svc.GetServer(context.Background(), "org-1", "server-001")
	require.NoError(t, err)
	assert.Equal(t, "server-001", result.ID)
	assert.Equal(t, "api-server", result.Name)
}

func TestService_DeleteServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	// DeleteServer 在 WT 内先 scoped GetServerByIDForOrganization，再 DeleteServer
	uow.EXPECT().Gateway().Return(repo).Times(2)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetServerByIDForOrganization(gomock.Any(), "server-001", "org-1").Return(&domain.Server{ID: "server-001", OrganizationID: "org-1"}, nil)
	repo.EXPECT().DeleteServer(gomock.Any(), "server-001").Return(nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	err := svc.DeleteServer(context.Background(), "org-1", "server-001")
	require.NoError(t, err)
}

func TestService_ListServers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo)
	repo.EXPECT().ListServers(gomock.Any(), gomock.Any()).Return([]*domain.Server{
		{ID: "s1", Name: "server1", CreatedAt: now, UpdatedAt: now},
		{ID: "s2", Name: "server2", CreatedAt: now, UpdatedAt: now},
	}, int64(2), nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	items, total, err := svc.ListServers(context.Background(), ListServersQueryApp{OrganizationID: "org-1"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
}

func TestService_GetSite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-001", "org-1").Return(&domain.Site{
		ID: "site-001", OrganizationID: "org-1", Name: "main-site",
		Host: "example.com", Status: "active", Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}, nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	result, err := svc.GetSite(context.Background(), "org-1", "site-001")
	require.NoError(t, err)
	assert.Equal(t, "site-001", result.ID)
	assert.Equal(t, "main-site", result.Name)
}

func TestService_DeleteSite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	// DeleteSite 在 WT 内先 scoped GetSiteByIDForOrganization，再 DeleteSite
	uow.EXPECT().Gateway().Return(repo).Times(2)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-001", "org-1").Return(&domain.Site{ID: "site-001", OrganizationID: "org-1"}, nil)
	repo.EXPECT().DeleteSite(gomock.Any(), "site-001").Return(nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	err := svc.DeleteSite(context.Background(), "org-1", "site-001")
	require.NoError(t, err)
}

func TestService_ListSites(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo)
	repo.EXPECT().ListSites(gomock.Any(), gomock.Any()).Return([]*domain.Site{
		{ID: "s1", Name: "site1", CreatedAt: now, UpdatedAt: now},
	}, int64(1), nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	items, total, err := svc.ListSites(context.Background(), ListSitesQueryApp{OrganizationID: "org-1"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
}

func TestService_UpdateServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetServerByIDForOrganization(gomock.Any(), "server-001", "org-1").Return(&domain.Server{
		ID: "server-001", OrganizationID: "org-1", Name: "api-server",
		Endpoint: "https://api.example.com", Protocol: "https",
		Status: "active", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}, nil)
	repo.EXPECT().UpdateServer(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	err := svc.UpdateServer(context.Background(), "org-1", UpdateServerCommand{
		ID:       "server-001",
		Name:     "updated-server",
		Endpoint: "https://api2.example.com",
		Protocol: "https",
		Status:   "inactive",
	})
	require.NoError(t, err)
}

func TestService_UpdateSite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-001", "org-1").Return(&domain.Site{
		ID: "site-001", OrganizationID: "org-1", Name: "main-site",
		Host: "example.com", Status: "active", Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}, nil)
	repo.EXPECT().UpdateSite(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	err := svc.UpdateSite(context.Background(), "org-1", UpdateSiteCommand{
		ID:     "site-001",
		Name:   "updated-site",
		Status: "inactive",
	})
	require.NoError(t, err)
}
