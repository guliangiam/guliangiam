package invitations

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// deliverInvitationEmail sends the invitation code to the invited address
// through the organization's resolved email sender (BE-A-02). A delivery
// failure surfaces as a classified application error (503 for an unclassified
// sender outage) so the HTTP caller never sees a silent "sent" success.
//
// When no resolver is wired the send is skipped with a WARN: embedders without
// messaging keep the legacy behaviour, but operators can see that delivery did
// not happen.
func (s *Service) deliverInvitationEmail(ctx context.Context, invitation *domain.Invitation) error {
	if s.SenderResolver == nil {
		slog.WarnContext(ctx, "invitation email delivery skipped: no sender resolver wired",
			"invitation_id", invitation.ID,
			"organization_id", invitation.OrganizationID,
		)
		return nil
	}

	sender, err := s.SenderResolver.ResolveEmailSender(ctx, invitation.OrganizationID)
	if err != nil {
		return err
	}
	if sender == nil {
		slog.WarnContext(ctx, "invitation email delivery skipped: resolver returned no sender",
			"invitation_id", invitation.ID,
			"organization_id", invitation.OrganizationID,
		)
		return nil
	}

	organizationLabel := invitation.OrganizationID
	if s.UOW.Organizations() != nil {
		if org, orgErr := s.UOW.Organizations().GetByID(ctx, invitation.OrganizationID); orgErr == nil && org != nil && org.Slug != "" {
			organizationLabel = org.Slug
		}
	}

	subject := "You are invited to join GuliangIAM"
	body := fmt.Sprintf(
		"You have been invited to join organization %q on GuliangIAM.\n\n"+
			"Invitation code: %s\n"+
			"Expires at: %s\n\n"+
			"Enter this code on the invitation registration page to complete sign-up.",
		organizationLabel, invitation.Code, invitation.ExpiresAt.UTC().Format(time.RFC3339),
	)

	if err := sender.Send(ctx, invitation.Email, subject, body); err != nil {
		var appErr *apperrors.Error
		if errors.As(err, &appErr) {
			return appErr
		}
		return apperrors.WrapUnavailable(err, "invitation email delivery is unavailable")
	}
	return nil
}
