package plans

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/plans"
	plansmock "github.com/guliangiam/guliangiam/internal/domain/plans/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertPlanInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertPlanNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestPlanByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the app layer rejects before touching any repository.
func TestPlanByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetPlan(ctx, "  ", "plan-1")
	assertPlanInvalidArgument(t, err)

	err = svc.PatchPlan(ctx, "", PatchPlanCommand{ID: "plan-1"})
	assertPlanInvalidArgument(t, err)

	err = svc.DeletePlan(ctx, "", "plan-1")
	assertPlanInvalidArgument(t, err)
}

func TestGetPlan_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := plansmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Plans().Return(repo)
	repo.EXPECT().GetByID(gomock.Any(), "plan-1", "org-1").
		Return(&plans.Plan{ID: "plan-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetPlan(context.Background(), "org-1", "plan-1")
	require.NoError(t, err)
}

func TestPatchPlan_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := plansmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Plans().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "plan-1", "org-1").
		Return(nil, apperrors.NewNotFound("plan not found"))
	// No Update expectation: a foreign id must never reach the write.

	name := "hijacked"
	err := svc.PatchPlan(context.Background(), "org-1", PatchPlanCommand{ID: "plan-1", DisplayName: &name})
	assertPlanNotFound(t, err)
}

func TestDeletePlan_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := plansmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Plans().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "plan-1", "org-1").
		Return(nil, apperrors.NewNotFound("plan not found"))
	// No Delete expectation: a foreign id must never reach the delete.

	err := svc.DeletePlan(context.Background(), "org-1", "plan-1")
	assertPlanNotFound(t, err)
}

func TestDeletePlan_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := plansmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Plans().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "plan-1", "org-1").
		Return(&plans.Plan{ID: "plan-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "plan-1", "org-1").Return(nil)

	require.NoError(t, svc.DeletePlan(context.Background(), "org-1", "plan-1"))
}
