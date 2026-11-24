package faceidproviders

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	faceidmock "github.com/guliangiam/guliangiam/internal/domain/faceidproviders/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertFaceIdInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertFaceIdNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestFaceIdProviderByID_RequiresOrganization pins the non-HTTP fail-fast:
// without a trusted tenant the app layer rejects before touching any repository.
func TestFaceIdProviderByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetFaceIdProvider(ctx, "  ", "provider-1")
	assertFaceIdInvalidArgument(t, err)

	err = svc.PatchFaceIdProvider(ctx, "", PatchFaceIdProviderCommand{ID: "provider-1"})
	assertFaceIdInvalidArgument(t, err)

	err = svc.DeleteFaceIdProvider(ctx, "", "provider-1")
	assertFaceIdInvalidArgument(t, err)
}

// TestGetFaceIdProvider_PassesTrustedOrganization pins the exact tenant
// argument seen by the repository.
func TestGetFaceIdProvider_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := faceidmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().FaceIdProviders().Return(repo)
	repo.EXPECT().GetByID(gomock.Any(), "provider-1", "org-1").
		Return(&faceidproviders.FaceIdProvider{ID: "provider-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetFaceIdProvider(context.Background(), "org-1", "provider-1")
	require.NoError(t, err)
}

// TestPatchFaceIdProvider_ForeignTenantStopsBeforeUpdate pins that ownership is
// decided before mutation: a NotFound read leaves Update uncalled.
func TestPatchFaceIdProvider_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := faceidmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().FaceIdProviders().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "provider-1", "org-1").
		Return(nil, apperrors.NewNotFound("face id provider not found"))
	// No Update expectation: a foreign id must never reach the write.

	name := "hijacked"
	err := svc.PatchFaceIdProvider(context.Background(), "org-1", PatchFaceIdProviderCommand{ID: "provider-1", Name: &name})
	assertFaceIdNotFound(t, err)
}

// TestDeleteFaceIdProvider_ForeignTenantStopsBeforeDelete pins that a foreign id
// is rejected before Delete.
func TestDeleteFaceIdProvider_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := faceidmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().FaceIdProviders().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "provider-1", "org-1").
		Return(nil, apperrors.NewNotFound("face id provider not found"))
	// No Delete expectation: a foreign id must never reach the delete.

	err := svc.DeleteFaceIdProvider(context.Background(), "org-1", "provider-1")
	assertFaceIdNotFound(t, err)
}

// TestDeleteFaceIdProvider_PassesEntityOrganizationToDelete pins the successful
// same-tenant delete: both the pre-read and the delete use the trusted tenant.
func TestDeleteFaceIdProvider_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := faceidmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().FaceIdProviders().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "provider-1", "org-1").
		Return(&faceidproviders.FaceIdProvider{ID: "provider-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "provider-1", "org-1").Return(nil)

	require.NoError(t, svc.DeleteFaceIdProvider(context.Background(), "org-1", "provider-1"))
}
