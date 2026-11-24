package pricings

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/pricings"
	pricingsmock "github.com/guliangiam/guliangiam/internal/domain/pricings/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertPricingInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertPricingNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestPricingByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the app layer rejects before touching any repository.
func TestPricingByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetPricing(ctx, "  ", "pricing-1")
	assertPricingInvalidArgument(t, err)

	err = svc.PatchPricing(ctx, "", PatchPricingCommand{ID: "pricing-1"})
	assertPricingInvalidArgument(t, err)

	err = svc.DeletePricing(ctx, "", "pricing-1")
	assertPricingInvalidArgument(t, err)
}

func TestGetPricing_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := pricingsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Pricings().Return(repo)
	repo.EXPECT().GetByID(gomock.Any(), "pricing-1", "org-1").
		Return(&pricings.Pricing{ID: "pricing-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetPricing(context.Background(), "org-1", "pricing-1")
	require.NoError(t, err)
}

func TestPatchPricing_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := pricingsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Pricings().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "pricing-1", "org-1").
		Return(nil, apperrors.NewNotFound("pricing not found"))
	// No Update expectation: a foreign id must never reach the write.

	name := "hijacked"
	err := svc.PatchPricing(context.Background(), "org-1", PatchPricingCommand{ID: "pricing-1", DisplayName: &name})
	assertPricingNotFound(t, err)
}

func TestDeletePricing_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := pricingsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Pricings().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "pricing-1", "org-1").
		Return(nil, apperrors.NewNotFound("pricing not found"))
	// No Delete expectation: a foreign id must never reach the delete.

	err := svc.DeletePricing(context.Background(), "org-1", "pricing-1")
	assertPricingNotFound(t, err)
}

func TestDeletePricing_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := pricingsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Pricings().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "pricing-1", "org-1").
		Return(&pricings.Pricing{ID: "pricing-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "pricing-1", "org-1").Return(nil)

	require.NoError(t, svc.DeletePricing(context.Background(), "org-1", "pricing-1"))
}
