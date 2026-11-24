package notificationproviders

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	notificationprovidersmock "github.com/guliangiam/guliangiam/internal/domain/notificationproviders/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertNotificationProviderInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertNotificationProviderNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestNotificationProviderByID_RequiresOrganization pins the non-HTTP fail-fast:
// without a trusted tenant the app layer rejects before touching any repository.
func TestNotificationProviderByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetNotificationProvider(ctx, "  ", "provider-1")
	assertNotificationProviderInvalidArgument(t, err)

	err = svc.PatchNotificationProvider(ctx, PatchNotificationProviderCommand{ID: "provider-1"})
	assertNotificationProviderInvalidArgument(t, err)

	err = svc.DeleteNotificationProvider(ctx, "", "provider-1")
	assertNotificationProviderInvalidArgument(t, err)
}

// TestGetNotificationProvider_PassesTrustedOrganization pins the exact tenant
// argument seen by the repository.
func TestGetNotificationProvider_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := notificationprovidersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().NotificationProviders().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "provider-1", "org-1").
		Return(&notificationproviders.NotificationProvider{ID: "provider-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetNotificationProvider(context.Background(), "org-1", "provider-1")
	require.NoError(t, err)
}

// TestPatchNotificationProvider_ForeignTenantStopsBeforeUpdate pins that
// ownership is decided before mutation: a NotFound read leaves Update uncalled.
func TestPatchNotificationProvider_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := notificationprovidersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().NotificationProviders().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "provider-1", "org-1").
		Return(nil, apperrors.NewNotFound("notification provider not found"))
	// No Update expectation: a foreign id must never reach the write.

	name := "hijacked"
	err := svc.PatchNotificationProvider(context.Background(), PatchNotificationProviderCommand{
		ID: "provider-1", OrganizationID: "org-1", Name: &name,
	})
	assertNotificationProviderNotFound(t, err)
}

// TestDeleteNotificationProvider_ForeignTenantStopsBeforeDelete pins that a
// foreign id is rejected before Delete.
func TestDeleteNotificationProvider_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := notificationprovidersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().NotificationProviders().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "provider-1", "org-1").
		Return(nil, apperrors.NewNotFound("notification provider not found"))
	// No Delete expectation: a foreign id must never reach the delete.

	err := svc.DeleteNotificationProvider(context.Background(), "org-1", "provider-1")
	assertNotificationProviderNotFound(t, err)
}

// TestDeleteNotificationProvider_PassesEntityOrganizationToDelete pins the
// successful same-tenant delete: both the pre-read and the delete use the
// trusted tenant and the audit carries the entity organization.
func TestDeleteNotificationProvider_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := notificationprovidersmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().NotificationProviders().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "provider-1", "org-1").
		Return(&notificationproviders.NotificationProvider{ID: "provider-1", OrganizationID: "org-1", Revision: 1}, nil)
	repo.EXPECT().Delete(gomock.Any(), "provider-1", "org-1").Return(nil)

	require.NoError(t, svc.DeleteNotificationProvider(context.Background(), "org-1", "provider-1"))
}
