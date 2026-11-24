package radiusconfigs

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	radiusmock "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertRadiusInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertRadiusNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestRadiusConfigByID_RequiresOrganization pins the non-HTTP fail-fast:
// without a trusted tenant the app layer rejects before touching any repository.
func TestRadiusConfigByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetRadiusConfig(ctx, "  ", "config-1")
	assertRadiusInvalidArgument(t, err)

	err = svc.PatchRadiusConfig(ctx, "", PatchRadiusConfigCommand{ID: "config-1"})
	assertRadiusInvalidArgument(t, err)

	err = svc.DeleteRadiusConfig(ctx, "", "config-1")
	assertRadiusInvalidArgument(t, err)
}

// TestGetRadiusConfig_PassesTrustedOrganization pins the exact tenant argument
// seen by the repository.
func TestGetRadiusConfig_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := radiusmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().RadiusConfigs().Return(repo)
	repo.EXPECT().GetByID(gomock.Any(), "config-1", "org-1").
		Return(&radiusconfigs.RadiusConfig{ID: "config-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetRadiusConfig(context.Background(), "org-1", "config-1")
	require.NoError(t, err)
}

// TestPatchRadiusConfig_ForeignTenantStopsBeforeUpdate pins that ownership is
// decided before mutation: a NotFound read leaves Update uncalled.
func TestPatchRadiusConfig_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := radiusmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().RadiusConfigs().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "config-1", "org-1").
		Return(nil, apperrors.NewNotFound("radius config not found"))
	// No Update expectation: a foreign id must never reach the write.

	name := "hijacked"
	err := svc.PatchRadiusConfig(context.Background(), "org-1", PatchRadiusConfigCommand{ID: "config-1", Name: &name})
	assertRadiusNotFound(t, err)
}

// TestDeleteRadiusConfig_ForeignTenantStopsBeforeDelete pins that a foreign id
// is rejected before Delete.
func TestDeleteRadiusConfig_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := radiusmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().RadiusConfigs().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "config-1", "org-1").
		Return(nil, apperrors.NewNotFound("radius config not found"))
	// No Delete expectation: a foreign id must never reach the delete.

	err := svc.DeleteRadiusConfig(context.Background(), "org-1", "config-1")
	assertRadiusNotFound(t, err)
}

// TestDeleteRadiusConfig_PassesEntityOrganizationToDelete pins the successful
// same-tenant delete: both the pre-read and the delete use the trusted tenant.
func TestDeleteRadiusConfig_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := radiusmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().RadiusConfigs().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "config-1", "org-1").
		Return(&radiusconfigs.RadiusConfig{ID: "config-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "config-1", "org-1").Return(nil)

	require.NoError(t, svc.DeleteRadiusConfig(context.Background(), "org-1", "config-1"))
}
