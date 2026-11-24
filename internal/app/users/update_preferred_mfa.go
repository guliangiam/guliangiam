package users

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type UpdateUserPreferredMfaCommand struct {
	OrganizationID string
	UserID         string
	MfaType        string
	Revision       int64
}

func (s *Service) UpdateUserPreferredMfa(ctx context.Context, command UpdateUserPreferredMfaCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if err := validateUpdateUserPreferredMfaCommand(command); err != nil {
		return err
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}

		if command.Revision > 0 && user.Revision != command.Revision {
			return apperrors.NewConflict("user revision mismatch")
		}

		mfaType := strings.TrimSpace(command.MfaType)
		matchedEnabledMethod := false
		for _, method := range user.MfaMethods {
			if method.MethodType == mfaType && method.Enabled {
				matchedEnabledMethod = true
				break
			}
		}
		if !matchedEnabledMethod {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "preferred MFA must reference an enabled MFA method", map[string]any{metaKeyField: "mfaType", metaKeyValue: mfaType})
		}

		user.PreferredMfaType = mfaType
		user.UpdatedAt = s.Clock.Now()
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, command.OrganizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.mfa.preferred.updated",
			"User",
			user.ID,
			"user",
			"user.mfa.preferred.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
