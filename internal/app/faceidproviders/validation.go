package faceidproviders

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateCreateFaceIdProviderCommand(command CreateFaceIdProviderCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(command.ProviderType) == "" {
		return apperrors.NewInvalidArgument("providerType is required")
	}
	return nil
}

func validatePatchFaceIdProviderCommand(command PatchFaceIdProviderCommand) error {
	if command.Name != nil && strings.TrimSpace(*command.Name) == "" {
		return apperrors.NewInvalidArgument("name must not be blank")
	}
	if command.ProviderType != nil && strings.TrimSpace(*command.ProviderType) == "" {
		return apperrors.NewInvalidArgument("providerType must not be blank")
	}
	return nil
}
