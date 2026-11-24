package captchaproviders

import (
	"context"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/captchaproviders/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func expectCaptchaUOWTransaction(uow *appuowmock.MockUnitOfWork) {
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
}

// TestCaptchaProviderByID_EmptyOrganizationRejected pins the app-layer fail-fast
// for non-HTTP callers that bypass the transport guard.
func TestCaptchaProviderByID_EmptyOrganizationRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().CaptchaProviders().Return(repo).AnyTimes()
	svc := NewService(fakeClock{}, fakeUUIDGenerator{}, uow)

	_, err := svc.GetCaptchaProvider(context.Background(), "", "provider-1")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)

	err = svc.PatchCaptchaProvider(context.Background(), PatchCaptchaProviderCommand{ID: "provider-1"})
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)

	err = svc.DeleteCaptchaProvider(context.Background(), "", "provider-1")
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

// TestGetCaptchaProvider_UsesTrustedOrganization pins the exact tenant argument
// handed to the repository.
func TestGetCaptchaProvider_UsesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().CaptchaProviders().Return(repo)
	repo.EXPECT().GetByID(gomock.Any(), "provider-1", "org-trusted").
		Return(&domain.CaptchaProvider{ID: "provider-1", OrganizationID: "org-trusted"}, nil)

	svc := NewService(fakeClock{}, fakeUUIDGenerator{}, uow)
	detail, err := svc.GetCaptchaProvider(context.Background(), "org-trusted", "provider-1")
	require.NoError(t, err)
	assert.Equal(t, "org-trusted", detail.OrganizationID)
}

// TestDeleteCaptchaProvider_ForeignTenantNotFound pins that a foreign id stops
// at the scoped read: no delete, no audit, no distinguishable error.
func TestDeleteCaptchaProvider_ForeignTenantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().CaptchaProviders().Return(repo).AnyTimes()
	expectCaptchaUOWTransaction(uow)
	repo.EXPECT().GetByID(gomock.Any(), "provider-1", "org-trusted").
		Return(nil, apperrors.NewNotFound("captcha provider not found"))
	// No Delete expectation: a foreign id must never reach the delete path.

	svc := NewService(fakeClock{}, fakeUUIDGenerator{}, uow)
	err := svc.DeleteCaptchaProvider(context.Background(), "org-trusted", "provider-1")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestDeleteCaptchaProvider_OwnTenantDeleteScoped pins that the delete path
// reads the scoped entity first and then deletes with the same tenant.
func TestDeleteCaptchaProvider_OwnTenantDeleteScoped(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().CaptchaProviders().Return(repo).AnyTimes()
	expectCaptchaUOWTransaction(uow)
	repo.EXPECT().GetByID(gomock.Any(), "provider-1", "org-trusted").
		Return(&domain.CaptchaProvider{ID: "provider-1", OrganizationID: "org-trusted", Revision: 1}, nil)
	repo.EXPECT().Delete(gomock.Any(), "provider-1", "org-trusted").Return(nil)

	svc := NewService(fakeClock{}, fakeUUIDGenerator{}, uow)
	require.NoError(t, svc.DeleteCaptchaProvider(context.Background(), "org-trusted", "provider-1"))
}
