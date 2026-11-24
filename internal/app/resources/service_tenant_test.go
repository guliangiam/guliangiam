package resources

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/resources"
	resourcesmock "github.com/guliangiam/guliangiam/internal/domain/resources/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertResourceInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertResourceNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestResourceByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the app layer rejects before touching any repository.
func TestResourceByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetResource(ctx, "  ", "resource-1")
	assertResourceInvalidArgument(t, err)

	err = svc.PatchResource(ctx, "", PatchResourceCommand{ID: "resource-1"})
	assertResourceInvalidArgument(t, err)

	err = svc.DeleteResource(ctx, "", "resource-1")
	assertResourceInvalidArgument(t, err)
}

func TestGetResource_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := resourcesmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Resources().Return(repo)
	repo.EXPECT().GetByID(gomock.Any(), "resource-1", "org-1").
		Return(&resources.Resource{ID: "resource-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetResource(context.Background(), "org-1", "resource-1")
	require.NoError(t, err)
}

func TestPatchResource_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := resourcesmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Resources().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "resource-1", "org-1").
		Return(nil, apperrors.NewNotFound("resource not found"))
	// No Update expectation: a foreign id must never reach the write.

	name := "hijacked"
	err := svc.PatchResource(context.Background(), "org-1", PatchResourceCommand{ID: "resource-1", DisplayName: &name})
	assertResourceNotFound(t, err)
}

func TestDeleteResource_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := resourcesmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Resources().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "resource-1", "org-1").
		Return(nil, apperrors.NewNotFound("resource not found"))
	// No Delete expectation: a foreign id must never reach the delete.

	err := svc.DeleteResource(context.Background(), "org-1", "resource-1")
	assertResourceNotFound(t, err)
}

func TestDeleteResource_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := resourcesmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Resources().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "resource-1", "org-1").
		Return(&resources.Resource{ID: "resource-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "resource-1", "org-1").Return(nil)

	require.NoError(t, svc.DeleteResource(context.Background(), "org-1", "resource-1"))
}
