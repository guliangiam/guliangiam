package bootstrap

import (
	"context"

	invitationsdomain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// The shared API fake unit of work lives in api_test.go. The tenant-scoped
// by-ID lookups are kept here so this batch does not rewrite that shared file.

func (r *fakeInvitationRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*invitationsdomain.Invitation, error) {
	invitation, ok := r.items[id]
	if !ok || invitation.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("invitation not found")
	}
	return invitation, nil
}

func (r *fakeVerificationRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*verificationsdomain.Verification, error) {
	verification, ok := r.items[id]
	if !ok || verification.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("verification not found")
	}
	return verification, nil
}
