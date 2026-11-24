package invitations

import (
	"context"

	invitationsdomain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// VerifyInvitationResult contains the invitation details after successful verification.
type VerifyInvitationResult struct {
	OrganizationID   string   `json:"organizationId"`
	OrganizationSlug string   `json:"organizationSlug,omitempty"`
	Email            string   `json:"email"`
	RoleIDs          []string `json:"roleIds"`
	GroupIDs         []string `json:"groupIds"`
	ExpiresAt        string   `json:"expiresAt"`
}

// VerifyInvitation checks if an invitation code is valid and returns the invitation details.
func (s *Service) VerifyInvitation(ctx context.Context, code string) (*VerifyInvitationResult, error) {
	if code == "" {
		return nil, apperrors.NewInvalidArgument("invitation code is required")
	}

	invitation, err := s.UOW.Invitations().GetByCode(ctx, code)
	if err != nil {
		return nil, apperrors.NewNotFound("invitation not found or invalid")
	}

	if !invitation.IsAcceptable(s.Clock.Now()) {
		if invitation.IsExpired(s.Clock.Now()) {
			return nil, apperrors.NewInvalidArgument("invitation has expired")
		}
		if invitation.Status == invitationsdomain.StatusAccepted {
			return nil, apperrors.NewConflict("invitation has already been accepted")
		}
		if invitation.Status == invitationsdomain.StatusRevoked {
			return nil, apperrors.NewInvalidArgument("invitation has been revoked")
		}
		return nil, apperrors.NewInvalidArgument("invitation is not in an acceptable state")
	}

	result := &VerifyInvitationResult{
		OrganizationID: invitation.OrganizationID,
		Email:          invitation.Email,
		RoleIDs:        invitation.RoleIDs,
		GroupIDs:       invitation.GroupIDs,
		ExpiresAt:      invitation.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	// 注册闭环需要组织 slug（signup 按 slug 定位组织）。组织行可见性与本请求
	// RLS 上下文一致：查不到就留空，不让公开校验端点因元数据缺失而失败。
	if org, err := s.UOW.Organizations().GetByID(ctx, invitation.OrganizationID); err == nil && org != nil {
		result.OrganizationSlug = org.Slug
	}
	return result, nil
}

// SendInvitation re-sends an invitation email for the invitation ID resolved
// in the trusted tenant.
func (s *Service) SendInvitation(ctx context.Context, organizationID, invitationID string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	invitation, err := s.UOW.Invitations().GetByIDForOrganization(ctx, invitationID, organizationID)
	if err != nil {
		return err
	}

	if invitation.Status != invitationsdomain.StatusPending {
		return apperrors.NewInvalidArgument("can only send pending invitations")
	}

	// Real delivery through the organization's email sender (BE-A-02). A
	// sender outage surfaces as 503 instead of a silent 200 "sent".
	return s.deliverInvitationEmail(ctx, invitation)
}
