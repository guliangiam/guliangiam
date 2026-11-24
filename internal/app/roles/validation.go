package roles

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateCreateRoleCommand(command CreateRoleCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName is required")
	}
	if err := validateUniqueStrings(command.GroupIDs, "groupIds", true); err != nil {
		return err
	}
	if err := validateUniqueStrings(command.RoleIDs, "roleIds", true); err != nil {
		return err
	}
	if err := validateUniqueStrings(command.Domains, "domains", true); err != nil {
		return err
	}

	return nil
}

func validatePatchRoleCommand(command PatchRoleCommand) error {
	if command.DisplayName != nil && strings.TrimSpace(*command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName must not be blank")
	}
	if command.Description != nil && strings.TrimSpace(*command.Description) == "" {
		return apperrors.NewInvalidArgument("description must not be blank")
	}
	if command.GroupIDs != nil {
		if err := validateUniqueStrings(command.GroupIDs, "groupIds", true); err != nil {
			return err
		}
	}
	if command.RoleIDs != nil {
		if err := validateUniqueStrings(command.RoleIDs, "roleIds", true); err != nil {
			return err
		}
	}
	if command.Domains != nil {
		if err := validateUniqueStrings(command.Domains, "domains", true); err != nil {
			return err
		}
	}

	return nil
}

func validateUniqueStrings(values []string, fieldName string, rejectBlank bool) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if rejectBlank && trimmed == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fieldName+" must not contain blank values", map[string]any{"field": fieldName})
		}
		if _, ok := seen[trimmed]; ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fieldName+" must not contain duplicates", map[string]any{"field": fieldName})
		}
		seen[trimmed] = struct{}{}
	}

	return nil
}
