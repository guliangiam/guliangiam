package adapters

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	adapterdomain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	adaptersmock "github.com/guliangiam/guliangiam/internal/domain/adapters/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ─── fakes ────────────────────────────────────────────────────────────────

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

// ─── CreateAdapter validation ─────────────────────────────────────────────

func TestCreateAdapter_MissingOrganizationID(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "adapter-001"},
	}
	err := svc.CreateAdapter(context.Background(), CreateAdapterCommand{
		Name: "pg-adapter",
	})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestCreateAdapter_MissingName(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "adapter-001"},
	}
	err := svc.CreateAdapter(context.Background(), CreateAdapterCommand{
		OrganizationID: "org-001",
	})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestCreateAdapter_WhitespaceOrganizationID(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "adapter-001"},
	}
	err := svc.CreateAdapter(context.Background(), CreateAdapterCommand{
		OrganizationID: "   ",
		Name:           "pg-adapter",
	})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestCreateAdapter_WhitespaceName(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "adapter-001"},
	}
	err := svc.CreateAdapter(context.Background(), CreateAdapterCommand{
		OrganizationID: "org-001",
		Name:           "   ",
	})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

// ─── CreateAdapter with mock UoW ──────────────────────────────────────────

func TestCreateAdapter_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)

	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "adapter-001"},
		UOW:   uow,
	}

	err := svc.CreateAdapter(context.Background(), CreateAdapterCommand{
		OrganizationID: "org-001",
		Name:           "pg-adapter",
		Type:           "database",
		Host:           "db.example.com",
	})
	require.NoError(t, err)
}

// ─── ListAdapters ─────────────────────────────────────────────────────────

func TestListAdapters_Empty(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*adapterdomain.Adapter{}, int64(0), nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	items, total, err := svc.ListAdapters(context.Background(), ListAdaptersQuery{})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestListAdapters_WithData(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(
		[]*adapterdomain.Adapter{
			{ID: "a1", OrganizationID: "org-1", Name: "pg", CreatedAt: now, UpdatedAt: now},
			{ID: "a2", OrganizationID: "org-1", Name: "mysql", CreatedAt: now, UpdatedAt: now},
		}, int64(2), nil,
	)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	items, total, err := svc.ListAdapters(context.Background(), ListAdaptersQuery{OrganizationID: "org-1"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
}

// ─── GetAdapter ───────────────────────────────────────────────────────────

func TestGetAdapter_NotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	repo.EXPECT().GetByID(gomock.Any(), "nonexistent", "org-1").Return(nil, apperrors.NewNotFound("adapter not found"))

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	_, err := svc.GetAdapter(context.Background(), "org-1", "nonexistent")
	require.Error(t, err)
}

func TestGetAdapter_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	repo.EXPECT().GetByID(gomock.Any(), "a1", "org-1").Return(&adapterdomain.Adapter{
		ID: "a1", OrganizationID: "org-1", Name: "pg", CreatedAt: now, UpdatedAt: now,
	}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	detail, err := svc.GetAdapter(context.Background(), "org-1", "a1")
	require.NoError(t, err)
	assert.Equal(t, "a1", detail.ID)
	assert.Equal(t, "pg", detail.Name)
}

// ─── PatchAdapter ─────────────────────────────────────────────────────────

func TestPatchAdapter_RevisionMismatch(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "a1", "org-1").Return(&adapterdomain.Adapter{ID: "a1", OrganizationID: "org-1", Revision: 2, CreatedAt: now, UpdatedAt: now}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.PatchAdapter(context.Background(), "org-1", PatchAdapterCommand{ID: "a1", Revision: 1})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
}

func TestPatchAdapter_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "a1", "org-1").Return(&adapterdomain.Adapter{ID: "a1", OrganizationID: "org-1", Revision: 1, CreatedAt: now, UpdatedAt: now}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	displayName := "New Name"
	err := svc.PatchAdapter(context.Background(), "org-1", PatchAdapterCommand{
		ID:          "a1",
		DisplayName: &displayName,
	})
	require.NoError(t, err)
}

// ─── DeleteAdapter ────────────────────────────────────────────────────────

func TestDeleteAdapter(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "a1", "org-1").Return(&adapterdomain.Adapter{ID: "a1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "a1", "org-1").Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.DeleteAdapter(context.Background(), "org-1", "a1")
	require.NoError(t, err)
}

// ─── mapper tests ─────────────────────────────────────────────────────────

func TestToListItem(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := &adapterdomain.Adapter{
		ID: "a-1", OrganizationID: "org-1", Name: "pg", DisplayName: "PostgreSQL",
		Type: "database", Server: "localhost", Database: "mydb",
		Owner: "admin", UseSameDb: true, IsEnabled: true,
		CreatedAt: now, UpdatedAt: now,
	}
	item := toListItem(a)
	assert.Equal(t, "a-1", item.ID)
	assert.Equal(t, "org-1", item.OrganizationID)
	assert.Equal(t, "pg", item.Name)
	assert.Equal(t, "PostgreSQL", item.DisplayName)
	assert.Equal(t, "database", item.Type)
	assert.True(t, item.IsEnabled)
}

func TestToDetail(t *testing.T) {
	t.Parallel()
	now := time.Now()
	port := 5432
	a := &adapterdomain.Adapter{
		ID: "a-1", OrganizationID: "org-1", Name: "pg", DisplayName: "PostgreSQL",
		Type: "database", Description: "Main DB", DatabaseType: "postgresql",
		Host: "db.example.com", Port: &port, DbUser: "admin", DbPassword: "secret",
		DatabaseName: "mydb", Policy: "", Revision: 2, CreatedAt: now, UpdatedAt: now,
	}
	detail := toDetail(a)
	assert.Equal(t, "a-1", detail.ID)
	assert.Equal(t, "Main DB", detail.Description)
	assert.Equal(t, "postgresql", detail.DatabaseType)
	assert.Equal(t, "db.example.com", detail.Host)
	assert.Equal(t, &port, detail.Port)
	assert.Equal(t, "admin", detail.DbUser)
	assert.True(t, detail.DbPasswordSet)
	assert.Equal(t, "*****", detail.DbPasswordMasked)
	assert.Equal(t, "mydb", detail.DatabaseName)
	assert.Equal(t, int64(2), detail.Revision)
}

func TestMaskSecret(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", maskSecret(""))
	assert.Equal(t, "*****", maskSecret("secret"))
}

// ─── DTO field tests ──────────────────────────────────────────────────────

func TestListAdaptersQuery_Fields(t *testing.T) {
	t.Parallel()
	q := ListAdaptersQuery{Page: 1, PageSize: 10, OrganizationID: "org-1", Keyword: "pg"}
	assert.Equal(t, 1, q.Page)
	assert.Equal(t, 10, q.PageSize)
	assert.Equal(t, "org-1", q.OrganizationID)
	assert.Equal(t, "pg", q.Keyword)
}

func TestPatchAdapterCommand_Fields(t *testing.T) {
	t.Parallel()
	displayName := "New Name"
	isEnabled := false
	cmd := PatchAdapterCommand{
		ID:          "a-1",
		DisplayName: &displayName,
		IsEnabled:   &isEnabled,
		Revision:    1,
	}
	assert.Equal(t, "a-1", cmd.ID)
	assert.NotNil(t, cmd.DisplayName)
	assert.Equal(t, "New Name", *cmd.DisplayName)
	assert.NotNil(t, cmd.IsEnabled)
	assert.False(t, *cmd.IsEnabled)
	assert.Equal(t, int64(1), cmd.Revision)
}

func TestCreateAdapterCommand_Fields(t *testing.T) {
	t.Parallel()
	port := 5432
	cmd := CreateAdapterCommand{
		OrganizationID: "org-1",
		Name:           "pg",
		Type:           "database",
		Host:           "db.example.com",
		Port:           &port,
		IsEnabled:      true,
	}
	assert.Equal(t, "org-1", cmd.OrganizationID)
	assert.Equal(t, "pg", cmd.Name)
	assert.Equal(t, &port, cmd.Port)
	assert.True(t, cmd.IsEnabled)
}
