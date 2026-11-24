package invitations

import (
	"context"
	"errors"
	"testing"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/invitations/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertInvitationInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertInvitationNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestGetInvitation_RequiresOrganization pins the non-HTTP fail-fast.
func TestGetInvitation_RequiresOrganization(t *testing.T) {
	svc := Service{}
	_, err := svc.GetInvitation(context.Background(), "  ", "inv-1")
	assertInvitationInvalidArgument(t, err)
}

// TestGetInvitation_PassesTrustedOrganization pins the scoped repository call.
func TestGetInvitation_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockIdentityUoW(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Invitations().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "inv-1", "org-1").
		Return(&domain.Invitation{ID: "inv-1", OrganizationID: "org-1"}, nil)

	dto, err := svc.GetInvitation(context.Background(), "org-1", "inv-1")
	require.NoError(t, err)
	require.Equal(t, "org-1", dto.OrganizationID)
}

// TestGetInvitation_ForeignTenantIsNotFound pins that a foreign row surfaces
// as not found and never falls back to the system-scoped GetByID.
func TestGetInvitation_ForeignTenantIsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockIdentityUoW(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Invitations().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "inv-1", "org-1").
		Return(nil, apperrors.NewNotFound("invitation not found"))

	_, err := svc.GetInvitation(context.Background(), "org-1", "inv-1")
	assertInvitationNotFound(t, err)
}
