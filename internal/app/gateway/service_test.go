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

// ─── fakes ────────────────────────────────────────────────────────────────

type fakeClock struct{ fixedTime time.Time }

func (f fakeClock) Now() time.Time { return f.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (f fakeUUIDGenerator) NewString() (string, error) { return f.fixedID, nil }

// ─── NewService ───────────────────────────────────────────────────────────

func TestNewService(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uow := uowmock.NewMockUnitOfWork(ctrl)

	svc := NewService(fakeClock{fixedTime: time.Now()}, fakeUUIDGenerator{fixedID: "test"}, uow)
	assert.NotNil(t, svc.Clock)
	assert.NotNil(t, svc.UUID)
	assert.Equal(t, uow, svc.UOW)
}

// ─── CreateServer ─────────────────────────────────────────────────────────

func TestService_CreateServer(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().CreateServer(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "server-001"},
		UOW:   uow,
	}

	result, err := svc.CreateServer(context.Background(), CreateServerCommand{
		OrganizationID: "org-1",
		Name:           "api-server",
		Endpoint:       "https://api.example.com",
		Protocol:       "https",
		Status:         "active",
	})
	require.NoError(t, err)
	assert.Equal(t, "server-001", result.ID)
	assert.Equal(t, "org-1", result.OrganizationID)
	assert.Equal(t, "api-server", result.Name)
	assert.Equal(t, "https://api.example.com", result.Endpoint)
	assert.Equal(t, int64(1), result.Revision)
}

// ─── CreateSite ───────────────────────────────────────────────────────────

func TestService_CreateSite(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().CreateSite(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "site-001"},
		UOW:   uow,
	}

	result, err := svc.CreateSite(context.Background(), CreateSiteCommand{
		OrganizationID:      "org-1",
		Name:                "main-site",
		Host:                "example.com",
		Status:              "active",
		SslMode:             "auto",
		ForceHttps:          true,
		EnableHealthCheck:   true,
		HealthCheckInterval: 30,
		EnableAlert:         true,
		AlertInterval:       60,
		AlertTryTimes:       3,
	})
	require.NoError(t, err)
	assert.Equal(t, "site-001", result.ID)
	assert.Equal(t, "org-1", result.OrganizationID)
	assert.Equal(t, "main-site", result.Name)
	assert.Equal(t, "example.com", result.Host)
	assert.Equal(t, "unknown", result.LastHealthStatus) // default
	assert.Equal(t, int64(1), result.Revision)
}

// ─── mapper tests ─────────────────────────────────────────────────────────

func TestToServerDTO_Nil(t *testing.T) {
	t.Parallel()
	result := toServerDTO(nil)
	assert.Nil(t, result)
}

func TestToServerDTO_Success(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := &domain.Server{
		ID: "s1", OrganizationID: "org-1", Name: "api-server",
		Endpoint: "https://api.example.com", Protocol: "https",
		Status: "active", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	dto := toServerDTO(s)
	assert.Equal(t, "s1", dto.ID)
	assert.Equal(t, "org-1", dto.OrganizationID)
	assert.Equal(t, "api-server", dto.Name)
	assert.Equal(t, "https://api.example.com", dto.Endpoint)
	assert.Equal(t, "https", dto.Protocol)
	assert.Equal(t, "active", dto.Status)
}

func TestToSiteDTO_Nil(t *testing.T) {
	t.Parallel()
	result := toSiteDTO(nil)
	assert.Nil(t, result)
}

func TestToSiteDTO_Success(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := &domain.Site{
		ID: "s1", OrganizationID: "org-1", Name: "main-site",
		Host: "example.com", Status: "active", SslMode: "auto",
		ForceHttps: true, EnableHealthCheck: true, HealthCheckInterval: 30,
		LastHealthStatus: "healthy", EnableAlert: true, AlertInterval: 60,
		AlertTryTimes: 3, AlertProviderIDs: []string{"p1", "p2"},
		Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	dto := toSiteDTO(s)
	assert.Equal(t, "s1", dto.ID)
	assert.Equal(t, "org-1", dto.OrganizationID)
	assert.Equal(t, "main-site", dto.Name)
	assert.Equal(t, "example.com", dto.Host)
	assert.Equal(t, "auto", dto.SslMode)
	assert.True(t, dto.ForceHttps)
	assert.True(t, dto.EnableHealthCheck)
	assert.Equal(t, 30, dto.HealthCheckInterval)
	assert.Equal(t, "healthy", dto.LastHealthStatus)
	assert.Equal(t, []string{"p1", "p2"}, dto.AlertProviderIDs)
}

// ─── DTO field tests ──────────────────────────────────────────────────────

func TestCreateServerCommand_Fields(t *testing.T) {
	t.Parallel()
	cmd := CreateServerCommand{
		OrganizationID: "org-1",
		Name:           "api-server",
		Endpoint:       "https://api.example.com",
		Protocol:       "https",
		Status:         "active",
	}
	assert.Equal(t, "org-1", cmd.OrganizationID)
	assert.Equal(t, "api-server", cmd.Name)
}

func TestCreateSiteCommand_Fields(t *testing.T) {
	t.Parallel()
	cmd := CreateSiteCommand{
		OrganizationID: "org-1",
		Name:           "main-site",
		Host:           "example.com",
		Status:         "active",
		SslMode:        "auto",
		ForceHttps:     true,
	}
	assert.Equal(t, "org-1", cmd.OrganizationID)
	assert.Equal(t, "main-site", cmd.Name)
	assert.True(t, cmd.ForceHttps)
}

func TestListServersQueryApp_Fields(t *testing.T) {
	t.Parallel()
	q := ListServersQueryApp{Page: 1, PageSize: 20, OrganizationID: "org-1", Status: "active"}
	assert.Equal(t, 1, q.Page)
	assert.Equal(t, 20, q.PageSize)
	assert.Equal(t, "org-1", q.OrganizationID)
}

func TestListSitesQueryApp_Fields(t *testing.T) {
	t.Parallel()
	q := ListSitesQueryApp{Page: 1, PageSize: 20, OrganizationID: "org-1", Status: "active"}
	assert.Equal(t, 1, q.Page)
	assert.Equal(t, 20, q.PageSize)
	assert.Equal(t, "org-1", q.OrganizationID)
}
