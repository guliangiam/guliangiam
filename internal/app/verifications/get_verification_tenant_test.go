package verifications

import (
	"context"
	"errors"
	"testing"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/verifications/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertVerificationInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertVerificationNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestGetVerification_RequiresOrganization pins the non-HTTP fail-fast.
func TestGetVerification_RequiresOrganization(t *testing.T) {
	svc := Service{}
	_, err := svc.GetVerification(context.Background(), "  ", "ver-1")
	assertVerificationInvalidArgument(t, err)
}

// TestGetVerification_PassesTrustedOrganization pins the scoped repository call.
func TestGetVerification_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Verifications().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "ver-1", "org-1").
		Return(&domain.Verification{ID: "ver-1", OrganizationID: "org-1"}, nil)

	dto, err := svc.GetVerification(context.Background(), "org-1", "ver-1")
	require.NoError(t, err)
	require.Equal(t, "org-1", dto.OrganizationID)
}

// TestGetVerification_ForeignTenantIsNotFound pins that a foreign row surfaces
// as not found and never falls back to the system-scoped GetByID.
func TestGetVerification_ForeignTenantIsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Verifications().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "ver-1", "org-1").
		Return(nil, apperrors.NewNotFound("verification not found"))

	_, err := svc.GetVerification(context.Background(), "org-1", "ver-1")
	assertVerificationNotFound(t, err)
}
