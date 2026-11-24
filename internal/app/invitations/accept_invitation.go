package invitations

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ValidateInvitationCode checks whether an invitation code is valid and returns
// the associated organization, email, and role/group information.
func (s *Service) ValidateInvitationCode(ctx context.Context, code string) (*ValidateInvitationCodeResult, error) {
	invitation, err := s.UOW.Invitations().GetByCode(ctx, code)
	if err != nil {
		// Unknown code is reported as "invalid" to avoid leaking invitation
		// existence. Any other failure is an infrastructure outage: surface it
		// as 503 instead of silently declaring every code invalid (BE-A-15).
		if apperrors.FromError(err).Code == apperrors.CodeNotFound {
			slog.DebugContext(ctx, "invitation code lookup returned not found", "error", err)
			return &ValidateInvitationCodeResult{Valid: false}, nil
		}
		return nil, apperrors.WrapUnavailable(err, "invitation code validation is unavailable")
	}

	now := s.Clock.Now()
	if !invitation.IsAcceptable(now) {
		return &ValidateInvitationCodeResult{Valid: false}, nil
	}

	return &ValidateInvitationCodeResult{
		Valid:          true,
		OrganizationID: invitation.OrganizationID,
		Email:          invitation.Email,
		RoleIDs:        invitation.RoleIDs,
		GroupIDs:       invitation.GroupIDs,
	}, nil
}

// Event types for invitation rejection audits. All follow
// "<aggregate>.<action>.<outcome>".
const (
	invitationEventAcceptNotFound    = "invitation.accept.not_found"
	invitationEventAcceptExpired     = "invitation.accept.expired"
	invitationEventAcceptStatusBad   = "invitation.accept.status_invalid"
	invitationEventRevokeNotFound    = "invitation.revoke.not_found"
	invitationEventRevokeAlreadyDone = "invitation.revoke.already_revoked"
	invitationEventResendNotFound    = "invitation.resend.not_found"
	invitationEventResendNotPending  = "invitation.resend.status_not_pending"
)

// AcceptInvitation marks an invitation as accepted.
func (s *Service) AcceptInvitation(ctx context.Context, command AcceptInvitationCommand) error {
	// 无 HTTP 入口；按 code 查询（GetByCode 无 organization_id 条件），可见性由调用方作用域（无会话时回落 built-in）与 RLS 决定。
	err := runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		invitation, err := tx.Invitations().GetByCode(ctx, command.Code)
		if err != nil {
			var invID string
			if invitation != nil {
				invID = invitation.ID
			}
			var orgID string
			if invitation != nil {
				orgID = invitation.OrganizationID
			}
			return &invitationTxReject{
				EventType: invitationEventAcceptNotFound,
				OrgID:     orgID,
				InvID:     invID,
				Payload:   map[string]any{"reason": "invitation_code_not_found"},
				Err:       apperrors.NewNotFound("invitation not found"),
			}
		}

		now := s.Clock.Now()
		if !invitation.IsAcceptable(now) {
			if invitation.IsExpired(now) {
				return &invitationTxReject{
					EventType: invitationEventAcceptExpired,
					OrgID:     invitation.OrganizationID,
					InvID:     invitation.ID,
					Payload: map[string]any{
						"expiresAt": invitation.ExpiresAt.UTC().Format(time.RFC3339),
					},
					Err: apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invitation has expired", map[string]any{metaKeyField: "expiresAt", metaKeyValue: invitation.ExpiresAt.UTC().Format(time.RFC3339), "expected": "future"}),
				}
			}
			return &invitationTxReject{
				EventType: invitationEventAcceptStatusBad,
				OrgID:     invitation.OrganizationID,
				InvID:     invitation.ID,
				Payload: map[string]any{
					"status": invitation.Status,
				},
				Err: apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("invitation cannot be accepted (status: %s)", invitation.Status), map[string]any{metaKeyField: "status", metaKeyValue: invitation.Status, "expected": domain.StatusPending}),
			}
		}

		invitation.Status = domain.StatusAccepted
		invitation.AcceptedAt = &now
		invitation.UpdatedAt = now
		// Update 的乐观锁守卫传自增前的 revision（仓储层 Revision-1），与 revoke/resend 一致。
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
