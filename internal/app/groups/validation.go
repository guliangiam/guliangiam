package groups

import (
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateCreateGroupCommand(command CreateGroupCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.Contains(command.Name, "/") {
		return apperrors.NewInvalidArgument("name can't contain \"/\"")
	}
	if strings.TrimSpace(command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName is required")
	}
	// 留空由 CreateGroup 回落 virtual；显式取值必须落在闭集内。
	if trimmed := strings.TrimSpace(command.Type); trimmed != "" && !domain.IsValidGroupType(trimmed) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"type must be one of virtual, physical, department",
			map[string]any{"field": "type", "value": trimmed, "allowed": []string{"virtual", "physical", "department"}})
	}
	return nil
}

func validatePatchGroupCommand(command PatchGroupCommand) error {
	if command.Name != nil {
		trimmed := strings.TrimSpace(*command.Name)
		if trimmed == "" {
			return apperrors.NewInvalidArgument("name must not be blank")
		}
		if strings.Contains(trimmed, "/") {
			return apperrors.NewInvalidArgument("name can't contain \"/\"")
		}
	}
	if command.DisplayName != nil && strings.TrimSpace(*command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName must not be blank")
	}
	if command.Type != nil {
		trimmed := strings.TrimSpace(*command.Type)
		if trimmed == "" {
			return apperrors.NewInvalidArgument("type must not be blank")
		}
		if !domain.IsValidGroupType(trimmed) {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"type must be one of virtual, physical, department",
				map[string]any{"field": "type", "value": trimmed, "allowed": []string{"virtual", "physical", "department"}})
		}
	}
	return nil
}
