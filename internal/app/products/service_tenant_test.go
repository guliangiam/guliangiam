package products

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/products"
	productsmock "github.com/guliangiam/guliangiam/internal/domain/products/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertProductInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertProductNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestProductByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the management app layer rejects before touching any repository.
func TestProductByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetProduct(ctx, "  ", "product-1")
	assertProductInvalidArgument(t, err)

	err = svc.PatchProduct(ctx, "", PatchProductCommand{ID: "product-1"})
	assertProductInvalidArgument(t, err)

	err = svc.DeleteProduct(ctx, "", "product-1")
	assertProductInvalidArgument(t, err)

	_, err = svc.BuyProduct(ctx, "", "product-1", "user", "", "", 1)
	assertProductInvalidArgument(t, err)
}

func TestGetProduct_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := productsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Products().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "product-1", "org-1").
		Return(&products.Product{ID: "product-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetProduct(context.Background(), "org-1", "product-1")
	require.NoError(t, err)
}

func TestPatchProduct_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := productsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Products().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "product-1", "org-1").
		Return(nil, apperrors.NewNotFound("product not found"))
	// No Update expectation: a foreign id must never reach the write.

	name := "hijacked"
	err := svc.PatchProduct(context.Background(), "org-1", PatchProductCommand{ID: "product-1", DisplayName: &name})
	assertProductNotFound(t, err)
}

func TestDeleteProduct_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := productsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Products().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "product-1", "org-1").
		Return(nil, apperrors.NewNotFound("product not found"))
	// No Delete expectation: a foreign id must never reach the delete.

	err := svc.DeleteProduct(context.Background(), "org-1", "product-1")
	assertProductNotFound(t, err)
}

func TestDeleteProduct_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := productsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Products().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "product-1", "org-1").
		Return(&products.Product{ID: "product-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "product-1", "org-1").Return(nil)

	require.NoError(t, svc.DeleteProduct(context.Background(), "org-1", "product-1"))
}

// TestBuyProduct_PassesTrustedOrganization pins that the purchase path is
// tenant-bound and never falls back to the system-scoped lookup.
func TestBuyProduct_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := productsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Products().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "product-1", "org-1").
		Return(&products.Product{ID: "product-1", OrganizationID: "org-1", State: products.StateActive}, nil)

	_, err := svc.BuyProduct(context.Background(), "org-1", "product-1", "user", "", "", 1)
	require.NoError(t, err)
}

// TestBuyProduct_RequiresOrganization pins the non-HTTP fail-fast.
func TestBuyProduct_RequiresOrganization(t *testing.T) {
	svc := Service{}
	_, err := svc.BuyProduct(context.Background(), "  ", "product-1", "user", "", "", 1)
	assertProductInvalidArgument(t, err)

	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := productsmock.NewMockRepository(ctrl)
	svc = Service{UOW: uow}
	uow.EXPECT().Products().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "product-1", "org-1").
		Return(nil, apperrors.NewNotFound("product not found"))

	_, err = svc.BuyProduct(context.Background(), "org-1", "product-1", "user", "", "", 1)
	assertProductNotFound(t, err)
}
