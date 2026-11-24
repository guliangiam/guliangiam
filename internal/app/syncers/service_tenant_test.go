package syncers

import (
	"context"
	"errors"
	"sync"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	syncersmock "github.com/guliangiam/guliangiam/internal/domain/syncers/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertSyncerInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertSyncerNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestSyncerByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the app layer rejects before touching any repository.
func TestSyncerByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetSyncer(ctx, "  ", "syncer-1")
	assertSyncerInvalidArgument(t, err)

	err = svc.PatchSyncer(ctx, "", PatchSyncerCommand{ID: "syncer-1"})
	assertSyncerInvalidArgument(t, err)

	err = svc.DeleteSyncer(ctx, "", "syncer-1")
	assertSyncerInvalidArgument(t, err)

	_, err = svc.RunSyncerForOrganization(ctx, "  ", "syncer-1")
	assertSyncerInvalidArgument(t, err)

	err = svc.TestConnectionForOrganization(ctx, "", "syncer-1")
	assertSyncerInvalidArgument(t, err)
}

// TestRunSyncer_ForeignTenantIsNotFound pins that the operational run path
// decides ownership before the running-guard or any engine call: a cross-tenant
// id is a plain NotFound, exactly like a missing row.
func TestRunSyncer_ForeignTenantIsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow, mu: &sync.Mutex{}, running: make(map[string]struct{})}

	uow.EXPECT().Syncers().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "syncer-1", "org-1").
		Return(nil, apperrors.NewNotFound("syncer not found"))

	_, err := svc.RunSyncerForOrganization(context.Background(), "org-1", "syncer-1")
	assertSyncerNotFound(t, err)
	require.Empty(t, svc.running, "a rejected run must not claim the running slot")
}

// TestRunSyncer_ForeignRowFailsClosed pins the app ownership gate even when a
// repository returns an unscoped row.
func TestRunSyncer_ForeignRowFailsClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow, mu: &sync.Mutex{}, running: make(map[string]struct{})}

	uow.EXPECT().Syncers().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "syncer-1", "org-1").
		Return(&domain.Syncer{ID: "syncer-1", OrganizationID: "org-2", IsEnabled: true}, nil)

	_, err := svc.RunSyncerForOrganization(context.Background(), "org-1", "syncer-1")
	assertSyncerNotFound(t, err)
	require.Empty(t, svc.running)
}

// TestTestConnection_ForeignTenantIsNotFound pins that the connection test
// cannot reach the engine with another tenant's stored credentials.
func TestTestConnection_ForeignTenantIsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Syncers().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "syncer-1", "org-1").
		Return(nil, apperrors.NewNotFound("syncer not found"))

	err := svc.TestConnectionForOrganization(context.Background(), "org-1", "syncer-1")
	assertSyncerNotFound(t, err)
}

// TestGetSyncer_PassesTrustedOrganization pins the exact tenant argument seen by
// the repository: it comes from the handler-resolved scope, not from the id.
func TestGetSyncer_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Syncers().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "syncer-1", "org-1").
		Return(&domain.Syncer{ID: "syncer-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetSyncer(context.Background(), "org-1", "syncer-1")
	require.NoError(t, err)
}

// TestGetSyncer_ForeignRowFailsClosed pins the application ownership gate: even
// if a repository implementation returned an unscoped row, a foreign tenant is
// reported as not found.
func TestGetSyncer_ForeignRowFailsClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Syncers().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "syncer-1", "org-1").
		Return(&domain.Syncer{ID: "syncer-1", OrganizationID: "org-2"}, nil)

	_, err := svc.GetSyncer(context.Background(), "org-1", "syncer-1")
	assertSyncerNotFound(t, err)
}

// TestPatchSyncer_ForeignTenantStopsBeforeUpdate pins that ownership is decided
// before mutation: a NotFound read leaves Update uncalled and writes no audit.
func TestPatchSyncer_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "syncer-1", "org-1").
		Return(nil, apperrors.NewNotFound("syncer not found"))
	// No Update expectation: a foreign id must never reach the write.

	displayName := "hijacked"
	err := svc.PatchSyncer(context.Background(), "org-1", PatchSyncerCommand{ID: "syncer-1", DisplayName: &displayName})
	assertSyncerNotFound(t, err)
}

// TestPatchSyncer_ForeignRowStopsBeforeUpdate pins the app ownership gate when a
// repository returns a row from another tenant.
func TestPatchSyncer_ForeignRowStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "syncer-1", "org-1").
		Return(&domain.Syncer{ID: "syncer-1", OrganizationID: "org-2", Revision: 1}, nil)

	displayName := "hijacked"
	err := svc.PatchSyncer(context.Background(), "org-1", PatchSyncerCommand{ID: "syncer-1", DisplayName: &displayName})
	assertSyncerNotFound(t, err)
}

// TestDeleteSyncer_ForeignTenantStopsBeforeDelete pins that a foreign id is
// rejected before Delete and the audit row carries the entity organization.
func TestDeleteSyncer_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "syncer-1", "org-1").
		Return(nil, apperrors.NewNotFound("syncer not found"))
	// No Delete expectation: a foreign id must never reach the delete.

	err := svc.DeleteSyncer(context.Background(), "org-1", "syncer-1")
	assertSyncerNotFound(t, err)
}

// TestDeleteSyncer_PassesEntityOrganizationToDelete pins the successful
// same-tenant delete: both the pre-read and the delete use the trusted tenant.
func TestDeleteSyncer_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := syncersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Syncers().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "syncer-1", "org-1").
		Return(&domain.Syncer{ID: "syncer-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "syncer-1", "org-1").Return(nil)

	require.NoError(t, svc.DeleteSyncer(context.Background(), "org-1", "syncer-1"))
}
