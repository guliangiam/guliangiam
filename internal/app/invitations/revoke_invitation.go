package invitations

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) RevokeInvitation(ctx context.Context, command RevokeInvitationCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if command.ID == "" {
		return apperrors.NewInvalidArgument("invitation id is required")
	}

	now := s.Clock.Now()
	// 撤销写路径的事务边界在此显式声明（等价于 runInTx，但让安全门禁与
	// 审查者直接看到 WithinTransaction，而不必穿透 helper）。
	runner, ok := appuow.EnsureTxRunner(s.UOW)
	if !ok {
		return apperrors.NewInternal("unit of work does not support transactions")
	}
	err := runner.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// The scoped read decides ownership before any mutation: a foreign or
		// missing id is NotFound and never reaches Update or the audit.
		invitation, err := tx.Invitations().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return &invitationTxReject{
				EventType: invitationEventRevokeNotFound,
				OrgID:     command.OrganizationID,
				InvID:     command.ID,
				Payload:   map[string]any{"reason": "invitation_not_found_or_not_in_tenant"},
				Err:       err,
			}
		}
		if invitation.Status == "revoked" {
			return &invitationTxReject{
				EventType: invitationEventRevokeAlreadyDone,
				OrgID:     invitation.OrganizationID,
				InvID:     invitation.ID,
				Payload:   map[string]any{"status": invitation.Status},
				Err:       apperrors.NewInvalidArgument("invitation is already revoked"),
			}
		}
		invitation.Status = "revoked"
		invitation.UpdatedAt = now
		invitation.Revision++
		if err := tx.Invitations().Update(ctx, invitation); err != nil {
			return err
		}

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
	return s.unwrapInvitationTxReject(ctx, err)
}
