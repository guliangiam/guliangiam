package invitations

import (
	"context"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
)

func (s *Service) CreateInvitation(ctx context.Context, command CreateInvitationCommand) (*domain.Invitation, error) {
	// HTTP handlers replace the body organization with TrustedOrganizationID;
	// non-HTTP callers must likewise supply an already-authorized tenant.
	if err := validateCreateInvitationCommand(command); err != nil {
		return nil, err
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	// Independent CSPRNG secret for user-facing URLs (BE-A-01): never derived
	// from the UUIDv7 row id, whose first characters are a predictable
	// millisecond timestamp.
	code, err := generateInvitationCode()
	if err != nil {
		return nil, err
	}

	// append([]string(nil), x...) 当 x 为 nil 时返回 nil，
	// repo 显式 INSERT role_ids/group_ids（ARRAY NOT NULL）→ 触发约束。
	// 必须显式做 nil→空数组 转换，与 organizations.NewOrganization 同构。
	roleIDs := command.RoleIDs
	if roleIDs == nil {
		roleIDs = []string{}
	}
	groupIDs := command.GroupIDs
	if groupIDs == nil {
		groupIDs = []string{}
	}

	now := s.Clock.Now()
	// 有效期：command.ValidDays 为 0 时沿用默认 7 天（校验层已限定 1..365）。
	validDays := command.ValidDays
	if validDays == 0 {
		validDays = 7
	}
	invitation := &domain.Invitation{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Email:          command.Email,
		Code:           code,
		RoleIDs:        roleIDs,
		GroupIDs:       groupIDs,
		Status:         domain.StatusPending,
		ExpiresAt:      now.Add(time.Duration(validDays) * 24 * time.Hour),
		InvitedBy:      command.InvitedBy,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	return invitation, runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Invitations().Create(ctx, invitation); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			invitation.OrganizationID,
			"invitations.create",
			invitation.ID,
			"invitations.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
