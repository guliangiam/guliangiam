package invitations

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	metaKeyField = "field"
	metaKeyValue = "value"
)

func validateCreateInvitationCommand(command CreateInvitationCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Email) == "" {
		return apperrors.NewInvalidArgument("email is required")
	}
	if command.ValidDays < 0 || command.ValidDays > 365 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "validDays must be between 1 and 365", map[string]any{
			metaKeyField: "validDays", metaKeyValue: command.ValidDays, "max": 365,
		})
	}
	// invited_by 是 uuid 列：非空必须是用户 ID，否则仓储直插会 500。
	if trimmed := strings.TrimSpace(command.InvitedBy); trimmed != "" && !platformuuid.IsValid(trimmed) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"invitedBy must be a user id (uuid)",
			map[string]any{metaKeyField: "invitedBy"},
		)
	}
	return nil
}
