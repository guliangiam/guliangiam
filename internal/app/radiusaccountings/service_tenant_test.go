package radiusaccountings

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	radiusaccountingsmock "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertRAInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertRANotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestRadiusAccountingByID_RequiresOrganization pins the non-HTTP fail-fast.
func TestRadiusAccountingByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetRadiusAccounting(ctx, "  ", "ra-1")
	assertRAInvalidArgument(t, err)

	err = svc.PatchRadiusAccounting(ctx, "", PatchRadiusAccountingCommand{ID: "ra-1"})
	assertRAInvalidArgument(t, err)

	err = svc.DeleteRadiusAccounting(ctx, "", "ra-1")
	assertRAInvalidArgument(t, err)
}

// TestGetRadiusAccounting_PassesTrustedOrganization pins the exact tenant
// argument seen by the repository.
func TestGetRadiusAccounting_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := radiusaccountingsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().RadiusAccountings().Return(repo)
	repo.EXPECT().GetByID(gomock.Any(), "ra-1", "org-1").
		Return(&domain.RadiusAccounting{ID: "ra-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetRadiusAccounting(context.Background(), "org-1", "ra-1")
	require.NoError(t, err)
}

// TestPatchRadiusAccounting_ForeignTenantStopsBeforeUpdate pins that ownership
// is decided before mutation: a NotFound read leaves Update uncalled.
func TestPatchRadiusAccounting_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := radiusaccountingsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().RadiusAccountings().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "ra-1", "org-1").
		Return(nil, apperrors.NewNotFound("radius accounting not found"))
	// No Update expectation: a foreign id must never reach the write.

	sessionTime := int64(60)
	err := svc.PatchRadiusAccounting(context.Background(), "org-1",
		PatchRadiusAccountingCommand{ID: "ra-1", AcctSessionTime: &sessionTime})
	assertRANotFound(t, err)
}

// TestDeleteRadiusAccounting_ForeignTenantStopsBeforeDelete pins the pre-read.
func TestDeleteRadiusAccounting_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := radiusaccountingsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().RadiusAccountings().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "ra-1", "org-1").
		Return(nil, apperrors.NewNotFound("radius accounting not found"))
	// No Delete expectation: a foreign id must never reach the delete.

	err := svc.DeleteRadiusAccounting(context.Background(), "org-1", "ra-1")
	assertRANotFound(t, err)
}

// TestDeleteRadiusAccounting_PassesEntityOrganizationToDelete pins that the
// successful delete uses the entity's real organization for both the pre-read
// and the delete predicate.
func TestDeleteRadiusAccounting_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := radiusaccountingsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().RadiusAccountings().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "ra-1", "org-1").
		Return(&domain.RadiusAccounting{ID: "ra-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "ra-1", "org-1").Return(nil)

	require.NoError(t, svc.DeleteRadiusAccounting(context.Background(), "org-1", "ra-1"))
}
