package ldaps

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	ldapsmock "github.com/guliangiam/guliangiam/internal/domain/ldaps/mock"
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

// ─── helpers ──────────────────────────────────────────────────────────────

// ─── NewService constructors ──────────────────────────────────────────────

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

func TestNewServiceWithScheduler(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uow := uowmock.NewMockUnitOfWork(ctrl)

	svc := NewServiceWithScheduler(fakeClock{fixedTime: time.Now()}, fakeUUIDGenerator{fixedID: "test"}, uow, nil)
	assert.Nil(t, svc.Scheduler)
}

// ─── CreateLdap validation (no UoW needed) ────────────────────────────────

func TestService_CreateLdap_MissingOrganizationID(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
	}
	err := svc.CreateLdap(context.Background(), CreateLdapCommand{Name: "test-ldap"})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestService_CreateLdap_MissingName(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
	}
	err := svc.CreateLdap(context.Background(), CreateLdapCommand{OrganizationID: "org-001"})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestService_CreateLdap_WhitespaceOrganizationID(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
	}
	err := svc.CreateLdap(context.Background(), CreateLdapCommand{OrganizationID: "   ", Name: "test-ldap"})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestService_CreateLdap_WhitespaceName(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
	}
	err := svc.CreateLdap(context.Background(), CreateLdapCommand{OrganizationID: "org-001", Name: "   "})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestService_CreateLdap_UUIDError(t *testing.T) {
	t.Parallel()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "", err: assert.AnError},
	}
	err := svc.CreateLdap(context.Background(), CreateLdapCommand{OrganizationID: "org-001", Name: "test-ldap", Host: "ldap.example.com", Port: 389})
	require.Error(t, err)
}

// ─── CreateLdap with mock UoW ─────────────────────────────────────────────

func TestService_CreateLdap_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)

	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	ldapRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "ldap-001"},
		UOW:   uow,
	}

	err := svc.CreateLdap(context.Background(), CreateLdapCommand{
		OrganizationID: "org-001",
		Name:           "test-ldap",
		Host:           "ldap.example.com",
		Port:           389,
	})
	require.NoError(t, err)
}

func TestService_CreateLdap_IsEnabledFalse(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)

	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	ldapRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	disabled := false
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "ldap-001"},
		UOW:   uow,
	}

	err := svc.CreateLdap(context.Background(), CreateLdapCommand{
		OrganizationID: "org-001",
		Name:           "test-ldap",
		Host:           "ldap.example.com",
		Port:           389,
		IsEnabled:      &disabled,
	})
	require.NoError(t, err)
}

// ─── ListLdaps ────────────────────────────────────────────────────────────

func TestService_ListLdaps_Empty(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	ldapRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domain.Ldap{}, int64(0), nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	items, total, err := svc.ListLdaps(context.Background(), ListLdapsQuery{})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestService_ListLdaps_WithData(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	ldapRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(
		[]*domain.Ldap{
			{ID: "l1", OrganizationID: "org-1", Name: "ldap1", CreatedAt: now, UpdatedAt: now},
			{ID: "l2", OrganizationID: "org-1", Name: "ldap2", CreatedAt: now, UpdatedAt: now},
		}, int64(2), nil,
	)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	items, total, err := svc.ListLdaps(context.Background(), ListLdapsQuery{OrganizationID: "org-1"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
}

// ─── GetLdap ──────────────────────────────────────────────────────────────

func TestService_GetLdap_NotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "nonexistent", "org-1").Return(nil, apperrors.NewNotFound("ldap not found"))

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	_, err := svc.GetLdap(context.Background(), "nonexistent", "org-1")
	require.Error(t, err)
}

func TestService_GetLdap_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "l1", "org-1").Return(&domain.Ldap{
		ID: "l1", OrganizationID: "org-1", Name: "ldap1",
		Host: "ldap.example.com", Port: 389, CreatedAt: now, UpdatedAt: now,
	}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	detail, err := svc.GetLdap(context.Background(), "l1", "org-1")
	require.NoError(t, err)
	assert.Equal(t, "l1", detail.ID)
	assert.Equal(t, "ldap1", detail.Name)
	assert.Equal(t, "ldap.example.com", detail.Host)
}

// ─── PatchLdap ────────────────────────────────────────────────────────────

func TestService_PatchLdap_RevisionMismatch(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "l1", "org-1").Return(&domain.Ldap{ID: "l1", OrganizationID: "org-1", Revision: 2, CreatedAt: now, UpdatedAt: now}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.PatchLdap(context.Background(), "org-1", PatchLdapCommand{ID: "l1", Revision: 1})
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
}

func TestService_PatchLdap_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "l1", "org-1").Return(&domain.Ldap{ID: "l1", OrganizationID: "org-1", Revision: 1, CreatedAt: now, UpdatedAt: now}, nil)
	ldapRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	displayName := "New Display"
	host := "new-host.example.com"
	err := svc.PatchLdap(context.Background(), "org-1", PatchLdapCommand{
		ID:          "l1",
		DisplayName: &displayName,
		Host:        &host,
	})
	require.NoError(t, err)
}

// ─── DeleteLdap ───────────────────────────────────────────────────────────

func TestService_DeleteLdap(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "l1", "org-1").Return(&domain.Ldap{ID: "l1", OrganizationID: "org-1"}, nil)
	ldapRepo.EXPECT().Delete(gomock.Any(), "l1", "org-1").Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.DeleteLdap(context.Background(), "org-1", "l1")
	require.NoError(t, err)
}

// ─── StartAutoSync / StopAutoSync ─────────────────────────────────────────

func TestService_StartAutoSync_NoScheduler(t *testing.T) {
	t.Parallel()
	svc := Service{}
	err := svc.StartAutoSync(context.Background(), "l1")
	assert.NoError(t, err)
}

func TestService_StopAutoSync_NoScheduler(t *testing.T) {
	t.Parallel()
	svc := Service{}
	svc.StopAutoSync("l1") // should not panic
}

func TestService_ScheduleAllEnabledAutoSyncs_NoScheduler(t *testing.T) {
	t.Parallel()
	svc := Service{}
	err := svc.ScheduleAllEnabledAutoSyncs(context.Background())
	assert.NoError(t, err)
}

// ─── TestConnection / PreviewUsers / SyncLdap (nil dependency paths) ───────

func TestService_TestConnection_NoClient(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "l1", "org-1").Return(&domain.Ldap{
		ID: "l1", OrganizationID: "org-1", Host: "ldap.example.com", Port: 389,
	}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.TestConnection(context.Background(), "l1", "org-1")
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnimplemented, appErr.Code)
}

func TestService_PreviewUsers_NoSyncer(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "l1", "org-1").Return(&domain.Ldap{ID: "l1", OrganizationID: "org-1"}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	_, err := svc.PreviewUsers(context.Background(), "l1", "org-1")
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnimplemented, appErr.Code)
}

func TestService_SyncLdap_NoSyncer(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "l1", "org-1").Return(&domain.Ldap{ID: "l1", OrganizationID: "org-1", IsEnabled: true}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	_, err := svc.SyncLdap(context.Background(), "l1", nil, "org-1")
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnimplemented, appErr.Code)
}

func TestService_SyncLdap_Disabled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "l1", "org-1").Return(&domain.Ldap{ID: "l1", OrganizationID: "org-1", IsEnabled: false}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	_, err := svc.SyncLdap(context.Background(), "l1", nil, "org-1")
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

// ─── mapper tests ─────────────────────────────────────────────────────────

func TestToListItem(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := &domain.Ldap{
		ID: "l1", OrganizationID: "org-1", Name: "ldap1", DisplayName: "LDAP 1",
		Host: "ldap.example.com", Port: 389, BaseDN: "dc=example,dc=com",
		IsEnabled: true, CreatedAt: now, UpdatedAt: now,
	}
	item := toListItem(l)
	assert.Equal(t, "l1", item.ID)
	assert.Equal(t, "org-1", item.OrganizationID)
	assert.Equal(t, "ldap1", item.Name)
	assert.Equal(t, "LDAP 1", item.DisplayName)
	assert.Equal(t, "ldap.example.com", item.Host)
	assert.Equal(t, 389, item.Port)
	assert.Equal(t, "dc=example,dc=com", item.BaseDN)
	assert.True(t, item.IsEnabled)
}

func TestToDetail(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := &domain.Ldap{
		ID: "l1", OrganizationID: "org-1", Name: "ldap1", DisplayName: "LDAP 1",
		Host: "ldap.example.com", Port: 389, BaseDN: "dc=example,dc=com",
		IsEnabled: true, Type: "OpenLDAP", AttributeName: "uid",
		PasswordType: "SSHA", Filter: "(objectClass=person)",
		FilterFields: []string{"uid", "mail"}, EnableSsl: true,
		AllowSelfSignedCert: false, EnableGroups: true,
		DefaultGroups: []string{"g1"}, CustomAttributes: map[string]string{"k": "v"},
		SyncInterval: 300, Revision: 2, CreatedAt: now, UpdatedAt: now,
	}
	detail := toDetail(l)
	assert.Equal(t, "l1", detail.ID)
	assert.Equal(t, "OpenLDAP", detail.Type)
	assert.Equal(t, "uid", detail.AttributeName)
	assert.Equal(t, "SSHA", detail.PasswordType)
	assert.Equal(t, "(objectClass=person)", detail.Filter)
	assert.True(t, detail.EnableSsl)
	assert.True(t, detail.EnableGroups)
	assert.Equal(t, []string{"g1"}, detail.DefaultGroups)
	assert.Equal(t, map[string]string{"k": "v"}, detail.CustomAttributes)
	assert.Equal(t, 300, detail.SyncInterval)
	assert.Equal(t, int64(2), detail.Revision)
}

// ─── buildSyncerConfig ────────────────────────────────────────────────────

func TestService_buildSyncerConfig(t *testing.T) {
	t.Parallel()
	svc := Service{}
	l := &domain.Ldap{
		ID: "l1", OrganizationID: "org-1", Host: "ldap.example.com", Port: 389,
		BindDN: "cn=admin,dc=example,dc=com", BindPassword: "secret",
		BaseDN: "dc=example,dc=com", Filter: "(objectClass=person)",
	}
	cfg := svc.buildSyncerConfig(l)
	assert.Equal(t, "ldap", cfg.ProviderType)
	assert.Equal(t, "ldap.example.com", cfg.Host)
	assert.Equal(t, 389, cfg.Port)
	assert.Equal(t, "cn=admin,dc=example,dc=com", cfg.Username)
	assert.Equal(t, "secret", cfg.Password)
	assert.Equal(t, "dc=example,dc=com", cfg.BaseDN)
	assert.Equal(t, "(objectClass=person)", cfg.Filter)
	assert.Equal(t, "org-1", cfg.OrganizationID)
}

// ─── DTO field tests ──────────────────────────────────────────────────────

func TestCreateLdapCommand_Fields(t *testing.T) {
	t.Parallel()
	enabled := true
	cmd := CreateLdapCommand{
		OrganizationID: "org-1", Name: "ldap1", Host: "ldap.example.com",
		Port: 389, IsEnabled: &enabled, SyncInterval: 300,
	}
	assert.Equal(t, "org-1", cmd.OrganizationID)
	assert.Equal(t, "ldap1", cmd.Name)
	assert.True(t, *cmd.IsEnabled)
	assert.Equal(t, 300, cmd.SyncInterval)
}

func TestPatchLdapCommand_Fields(t *testing.T) {
	t.Parallel()
	displayName := "New Name"
	isEnabled := false
	syncInterval := 600
	cmd := PatchLdapCommand{
		ID: "l1", DisplayName: &displayName, IsEnabled: &isEnabled,
		SyncInterval: &syncInterval, Revision: 1,
	}
	assert.Equal(t, "l1", cmd.ID)
	assert.Equal(t, "New Name", *cmd.DisplayName)
	assert.False(t, *cmd.IsEnabled)
	assert.Equal(t, 600, *cmd.SyncInterval)
	assert.Equal(t, int64(1), cmd.Revision)
}

func TestListLdapsQuery_Fields(t *testing.T) {
	t.Parallel()
	q := ListLdapsQuery{Page: 1, PageSize: 20, OrganizationID: "org-1", Keyword: "ldap"}
	assert.Equal(t, 1, q.Page)
	assert.Equal(t, 20, q.PageSize)
	assert.Equal(t, "org-1", q.OrganizationID)
	assert.Equal(t, "ldap", q.Keyword)
}
