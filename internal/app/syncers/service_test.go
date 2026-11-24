package syncers

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	syncersmock "github.com/guliangiam/guliangiam/internal/domain/syncers/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ─── fakes ────────────────────────────────────────────────────────────────

type fakeClock struct{ fixedTime time.Time }

func (f fakeClock) Now() time.Time { return f.fixedTime }

type fakeUUIDGenerator struct {
	fixedID string
	err     error
}

func (f fakeUUIDGenerator) NewString() (string, error) { return f.fixedID, f.err }

// ─── NewService constructors ──────────────────────────────────────────────

func TestNewService(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uow := uowmock.NewMockUnitOfWork(ctrl)

	svc := NewService(fakeClock{fixedTime: time.Now()}, fakeUUIDGenerator{fixedID: "test"}, uow)
	assert.NotNil(t, svc.Clock)
	assert.NotNil(t, svc.UUID)
	assert.Nil(t, svc.Factory)
}

func TestNewServiceWithFactory(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uow := uowmock.NewMockUnitOfWork(ctrl)

	svc := NewServiceWithFactory(fakeClock{fixedTime: time.Now()}, fakeUUIDGenerator{fixedID: "test"}, uow, nil)
	assert.Nil(t, svc.Factory)
}

// ─── CreateSyncer validation ──────────────────────────────────────────────

func TestService_CreateSyncer_MissingOrganizationID(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
	}
	err := svc.CreateSyncer(context.Background(), CreateSyncerCommand{Name: "test-syncer"})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestService_CreateSyncer_MissingName(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
	}
	err := svc.CreateSyncer(context.Background(), CreateSyncerCommand{OrganizationID: "org-001"})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestService_CreateSyncer_WhitespaceOrganizationID(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
	}
	err := svc.CreateSyncer(context.Background(), CreateSyncerCommand{OrganizationID: "   ", Name: "test-syncer"})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestService_CreateSyncer_WhitespaceName(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
	}
	err := svc.CreateSyncer(context.Background(), CreateSyncerCommand{OrganizationID: "org-001", Name: "   "})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestService_CreateSyncer_UUIDError(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "", err: assert.AnError},
	}
	err := svc.CreateSyncer(context.Background(), CreateSyncerCommand{OrganizationID: "org-001", Name: "test-syncer"})
	require.Error(t, err)
}

// ─── CreateSyncer with mock UoW ───────────────────────────────────────────

func TestService_CreateSyncer_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)

	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "syncer-001"},
		UOW:   uow,
	}

	err := svc.CreateSyncer(context.Background(), CreateSyncerCommand{
		OrganizationID: "org-001",
		Name:           "test-syncer",
		Host:           "db.example.com",
		Port:           5432,
		Type:           "database",
	})
	require.NoError(t, err)
}

func TestService_CreateSyncer_IsEnabledFalse(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)

	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	disabled := false
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "syncer-001"},
		UOW:   uow,
	}

	err := svc.CreateSyncer(context.Background(), CreateSyncerCommand{
		OrganizationID: "org-001",
		Name:           "test-syncer",
		IsEnabled:      &disabled,
	})
	require.NoError(t, err)
}

// ─── ListSyncers ──────────────────────────────────────────────────────────

func TestService_ListSyncers_Empty(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domain.Syncer{}, int64(0), nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	items, total, err := svc.ListSyncers(context.Background(), ListSyncersQuery{})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestService_ListSyncers_WithData(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(
		[]*domain.Syncer{
			{ID: "s1", OrganizationID: "org-1", Name: "syncer1", CreatedAt: now, UpdatedAt: now},
			{ID: "s2", OrganizationID: "org-1", Name: "syncer2", CreatedAt: now, UpdatedAt: now},
		}, int64(2), nil,
	)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	items, total, err := svc.ListSyncers(context.Background(), ListSyncersQuery{OrganizationID: "org-1"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
}

// ─── GetSyncer ────────────────────────────────────────────────────────────

func TestService_GetSyncer_NotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "nonexistent", "org-1").Return(nil, apperrors.NewNotFound("syncer not found"))

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	_, err := svc.GetSyncer(context.Background(), "org-1", "nonexistent")
	require.Error(t, err)
}

func TestService_GetSyncer_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "s1", "org-1").Return(&domain.Syncer{
		ID: "s1", OrganizationID: "org-1", Name: "syncer1",
		Host: "db.example.com", Port: 5432, CreatedAt: now, UpdatedAt: now,
	}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	detail, err := svc.GetSyncer(context.Background(), "org-1", "s1")
	require.NoError(t, err)
	assert.Equal(t, "s1", detail.ID)
	assert.Equal(t, "syncer1", detail.Name)
}

// ─── PatchSyncer ──────────────────────────────────────────────────────────

func TestService_PatchSyncer_RevisionMismatch(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "s1", "org-1").Return(&domain.Syncer{ID: "s1", OrganizationID: "org-1", Revision: 2, CreatedAt: now, UpdatedAt: now}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.PatchSyncer(context.Background(), "org-1", PatchSyncerCommand{ID: "s1", Revision: 1})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
}

func TestService_PatchSyncer_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "s1", "org-1").Return(&domain.Syncer{ID: "s1", OrganizationID: "org-1", Revision: 1, CreatedAt: now, UpdatedAt: now}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	displayName := "New Display"
	host := "new-host.example.com"
	err := svc.PatchSyncer(context.Background(), "org-1", PatchSyncerCommand{
		ID:          "s1",
		DisplayName: &displayName,
		Host:        &host,
	})
	require.NoError(t, err)
}

// ─── DeleteSyncer ─────────────────────────────────────────────────────────

func TestService_DeleteSyncer(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "s1", "org-1").Return(&domain.Syncer{ID: "s1", OrganizationID: "org-1", Name: "syncer1", Revision: 1}, nil)
	repo.EXPECT().Delete(gomock.Any(), "s1", "org-1").Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.DeleteSyncer(context.Background(), "org-1", "s1")
	require.NoError(t, err)
}

// ─── mapper tests ─────────────────────────────────────────────────────────

func TestToListItem(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := &domain.Syncer{
		ID: "s1", OrganizationID: "org-1", Name: "syncer1", DisplayName: "Syncer 1",
		Type: domain.SyncerTypeDatabase, Host: "db.example.com", Port: 5432,
		IsEnabled: true, SyncInterval: 300, CreatedAt: now, UpdatedAt: now,
	}
	item := toListItem(s)
	assert.Equal(t, "s1", item.ID)
	assert.Equal(t, "org-1", item.OrganizationID)
	assert.Equal(t, "syncer1", item.Name)
	assert.Equal(t, "database", item.Type)
	assert.Equal(t, "db.example.com", item.Host)
	assert.Equal(t, 5432, item.Port)
	assert.True(t, item.IsEnabled)
	assert.Equal(t, 300, item.SyncInterval)
}

func TestToDetail(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := &domain.Syncer{
		ID: "s1", OrganizationID: "org-1", Name: "syncer1", DisplayName: "Syncer 1",
		Type: domain.SyncerTypeDatabase, Host: "db.example.com", Port: 5432,
		IsEnabled: true, SyncInterval: 300,
		DatabaseType: "postgresql", Database: "mydb", TableName: "users",
		SslMode: "require", SshType: "password", SshHost: "ssh.example.com",
		SshPort: 22, SshUsername: "sshuser", SshCertID: "cert-1",
		FieldMapping: `{"username":"uid"}`, ColumnMapping: `{"name":"cn"}`,
		IsReadOnly: true, Affinity: "user", AffiliationTable: "user_group",
		AvatarBaseUrl: "https://avatars.example.com", ErrorText: "",
		Revision: 2, CreatedAt: now, UpdatedAt: now,
	}
	detail := toDetail(s)
	assert.Equal(t, "s1", detail.ID)
	assert.Equal(t, "postgresql", detail.DatabaseType)
	assert.Equal(t, "mydb", detail.Database)
	assert.Equal(t, "users", detail.TableName)
	assert.Equal(t, "require", detail.SslMode)
	assert.True(t, detail.IsReadOnly)
	assert.Equal(t, int64(2), detail.Revision)
}

// ─── DTO field tests ──────────────────────────────────────────────────────

func TestCreateSyncerCommand_Fields(t *testing.T) {
	t.Parallel()
	enabled := true
	cmd := CreateSyncerCommand{
		OrganizationID: "org-1", Name: "syncer1", Host: "db.example.com",
		Port: 5432, Type: "database", IsEnabled: &enabled, SyncInterval: 300,
	}
	assert.Equal(t, "org-1", cmd.OrganizationID)
	assert.True(t, *cmd.IsEnabled)
}

func TestPatchSyncerCommand_Fields(t *testing.T) {
	t.Parallel()
	displayName := "New Name"
	isEnabled := false
	cmd := PatchSyncerCommand{
		ID: "s1", DisplayName: &displayName, IsEnabled: &isEnabled, Revision: 1,
	}
	assert.Equal(t, "s1", cmd.ID)
	assert.Equal(t, "New Name", *cmd.DisplayName)
	assert.False(t, *cmd.IsEnabled)
}

func TestListSyncersQuery_Fields(t *testing.T) {
	t.Parallel()
	q := ListSyncersQuery{Page: 1, PageSize: 20, OrganizationID: "org-1", Keyword: "syncer"}
	assert.Equal(t, 1, q.Page)
	assert.Equal(t, 20, q.PageSize)
}

// ─── extractString helper ─────────────────────────────────────────────────

func TestExtractString(t *testing.T) {
	t.Parallel()
	m := map[string]interface{}{
		"username": "alice",
		"count":    42,
	}
	assert.Equal(t, "alice", extractString(m, "username"))
	assert.Equal(t, "42", extractString(m, "count"))
	assert.Equal(t, "", extractString(m, "missing"))
}

// ─── retryableSyncError ───────────────────────────────────────────────────

func TestRetryableSyncError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		expect bool
	}{
		{"nil error", nil, false},
		{"timeout", apperrors.NewInternal("connection timeout"), true},
		{"connection refused", apperrors.NewInternal("connection refused"), true},
		{"temporary", apperrors.NewInternal("temporary failure"), true},
		{"i/o timeout", apperrors.NewInternal("i/o timeout"), true},
		{"network", apperrors.NewInternal("network error"), true},
		{"non-retryable", apperrors.NewInternal("permission denied"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, retryableSyncError(tt.err))
		})
	}
}
