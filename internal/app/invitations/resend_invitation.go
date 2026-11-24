package invitations

import (
	"context"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) ResendInvitation(ctx context.Context, command ResendInvitationCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if command.ID == "" {
		return apperrors.NewInvalidArgument("invitation id is required")
	}

	now := s.Clock.Now()
	var resent *domain.Invitation
	err := runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// The scoped read decides ownership before any mutation: a foreign or
		// missing id is NotFound and never reaches Update or the audit.
		invitation, err := tx.Invitations().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return &invitationTxReject{
				EventType: invitationEventResendNotFound,
				OrgID:     command.OrganizationID,
				InvID:     command.ID,
				Payload:   map[string]any{"reason": "invitation_not_found_or_not_in_tenant"},
				Err:       err,
			}
		}
		if invitation.Status != "pending" {
			return &invitationTxReject{
				EventType: invitationEventResendNotPending,
				OrgID:     invitation.OrganizationID,
				InvID:     invitation.ID,
				Payload:   map[string]any{"currentStatus": invitation.Status, "expectedStatus": "pending"},
				Err:       apperrors.NewInvalidArgument("only pending invitations can be resent"),
			}
		}
		invitation.ExpiresAt = now.Add(7 * 24 * time.Hour)
		invitation.UpdatedAt = now
		invitation.Revision++
		if err := tx.Invitations().Update(ctx, invitation); err != nil {
			return err
		}
		resent = invitation

		if err := s.embedAuditEvent(
			tx,
			ctx,
			invitation.OrganizationID,
			"invitations.update",
			invitation.ID,
			"invitations.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	if err := s.unwrapInvitationTxReject(ctx, err); err != nil {
		return err
	}
	// Deliver after the expiry extension commits (BE-A-02). A delivery failure
	// is returned to the caller (503) so the resend is observable as failed;
	// the extended expiry is harmless and the next resend retries delivery.
	return s.deliverInvitationEmail(ctx, resent)
}
