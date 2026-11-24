package permissions

import (
	"context"
	"github.com/google/uuid"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateCreatePermissionCommand(command CreatePermissionCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName is required")
	}
	if strings.TrimSpace(command.ResourceType) == "" {
		return apperrors.NewInvalidArgument("resourceType is required")
	}
	if err := validateUniqueStrings(command.Users, "users"); err != nil {
		return err
	}
	if err := validateUniqueStrings(command.Groups, "groups"); err != nil {
		return err
	}
	if err := validateUniqueStrings(command.Roles, "roles"); err != nil {
		return err
	}
	if err := validateUniqueStrings(command.Domains, "domains"); err != nil {
		return err
	}
	if err := validateUniqueStrings(command.Resources, "resources"); err != nil {
		return err
	}
	if err := validateUniqueStrings(command.Actions, "actions"); err != nil {
		return err
	}
	if !isValidEffect(command.Effect) {
		return invalidEffectError(command.Effect)
	}
	if !isValidState(command.State) {
		return invalidStateError(command.State)
	}

	return nil
}

func validatePatchPermissionCommand(command PatchPermissionCommand) error {
	if command.DisplayName != nil && strings.TrimSpace(*command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName must not be blank")
	}
	if command.Description != nil && strings.TrimSpace(*command.Description) == "" {
		return apperrors.NewInvalidArgument("description must not be blank")
	}
	if command.ResourceType != nil && strings.TrimSpace(*command.ResourceType) == "" {
		return apperrors.NewInvalidArgument("resourceType must not be blank")
	}
	if command.Submitter != nil && strings.TrimSpace(*command.Submitter) == "" {
		return apperrors.NewInvalidArgument("submitter must not be blank")
	}
	if command.Approver != nil && strings.TrimSpace(*command.Approver) == "" {
		return apperrors.NewInvalidArgument("approver must not be blank")
	}
	if command.Users != nil {
		if err := validateUniqueStrings(command.Users, "users"); err != nil {
			return err
		}
	}
	if command.Groups != nil {
		if err := validateUniqueStrings(command.Groups, "groups"); err != nil {
			return err
		}
	}
	if command.Roles != nil {
		if err := validateUniqueStrings(command.Roles, "roles"); err != nil {
			return err
		}
	}
	if command.Domains != nil {
		if err := validateUniqueStrings(command.Domains, "domains"); err != nil {
			return err
		}
	}
	if command.Resources != nil {
		if err := validateUniqueStrings(command.Resources, "resources"); err != nil {
			return err
		}
	}
	if command.Actions != nil {
		if err := validateUniqueStrings(command.Actions, "actions"); err != nil {
			return err
		}
	}
	if command.Effect != nil && !isValidEffect(*command.Effect) {
		return invalidEffectError(*command.Effect)
	}
	if command.State != nil && !isValidState(*command.State) {
		return invalidStateError(*command.State)
	}

	return nil
}

func validateUniqueStrings(values []string, fieldName string) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fieldName+" must not contain blank values", map[string]any{"field": fieldName})
		}
		if _, ok := seen[trimmed]; ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fieldName+" must not contain duplicates", map[string]any{"field": fieldName})
		}
		seen[trimmed] = struct{}{}
	}

	return nil
}

func normalizeEffect(value string) string {
	if strings.TrimSpace(value) == "" {
		return "allow"
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeState(value string) string {
	if strings.TrimSpace(value) == "" {
		return "active"
	}
	return strings.ToLower(strings.TrimSpace(value))
}

// permissionEffects and permissionStates are the single source of the closed
// sets. isValidEffect/isValidState and the 422 metadata both read them, so a
// value cannot be accepted by the rule while being absent from `allowed`.
var permissionEffects = []string{"allow", "deny"}

var permissionStates = []string{"active", "pending", "approved", "rejected", "disabled"}

func isValidEffect(value string) bool {
	normalized := normalizeEffect(value)
	for _, allowed := range permissionEffects {
		if normalized == allowed {
			return true
		}
	}
	return false
}

func isValidState(value string) bool {
	normalized := normalizeState(value)
	for _, allowed := range permissionStates {
		if normalized == allowed {
			return true
		}
	}
	return false
}

func invalidEffectError(value string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"effect must be allow or deny",
		map[string]any{"field": "effect", "value": value, "allowed": append([]string{}, permissionEffects...)},
	)
}

func invalidStateError(value string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"state is invalid",
		map[string]any{"field": "state", "value": value, "allowed": append([]string{}, permissionStates...)},
	)
}

// validatePermissionResourcesForOrganization 在任何 Casbin 读取/写入前校验
// resources[0] 指向的 enforcer 与其 model 均归属可信租户（OPEN-API-44）。
// casbin_rules 无 organization 列：MySQL 无 RLS，只有这层归属裁决能阻止
// 租户把策略面指向他租户 enforcer。
func validatePermissionResourcesForOrganization(ctx context.Context, tx appuow.UnitOfWork, organizationID string, resources []string) error {
	if len(resources) == 0 {
		return nil
	}
	enforcerID := strings.TrimSpace(resources[0])
	if enforcerID == "" {
		return apperrors.NewInvalidArgument("permission has no associated enforcer")
	}
	if _, err := uuid.Parse(enforcerID); err != nil {
		return apperrors.NewNotFound("enforcer not found")
	}
	enforcer, err := tx.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(enforcer.ModelID) == "" {
		return apperrors.NewForbidden("enforcer has no model assigned")
	}
	if _, err := tx.Models().GetByIDForOrganization(ctx, enforcer.ModelID, organizationID); err != nil {
		return err
	}
	return nil
}
