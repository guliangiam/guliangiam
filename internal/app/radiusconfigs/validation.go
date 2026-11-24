package radiusconfigs

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateCreateRadiusConfigCommand(command CreateRadiusConfigCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(command.Host) == "" {
		return apperrors.NewInvalidArgument("host is required")
	}
	if strings.TrimSpace(command.AuthProtocol) == "" {
		return apperrors.NewInvalidArgument("authProtocol is required")
	}
	if err := validateRadiusPort(command.Port); err != nil {
		return err
	}
	return nil
}

func validatePatchRadiusConfigCommand(command PatchRadiusConfigCommand) error {
	if command.Name != nil && strings.TrimSpace(*command.Name) == "" {
		return apperrors.NewInvalidArgument("name must not be blank")
	}
	if command.Host != nil && strings.TrimSpace(*command.Host) == "" {
		return apperrors.NewInvalidArgument("host must not be blank")
	}
	if command.AuthProtocol != nil && strings.TrimSpace(*command.AuthProtocol) == "" {
		return apperrors.NewInvalidArgument("authProtocol must not be blank")
	}
	if command.Port != nil {
		if err := validateRadiusPort(*command.Port); err != nil {
			return err
		}
	}
	return nil
}

// validateRadiusPort enforces the UDP/TCP port range; port=0 used to be
// persisted and only failed later when the RADIUS server tried to listen.
func validateRadiusPort(port int) error {
	if port < 1 || port > 65535 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "port must be between 1 and 65535", map[string]any{
			"field": "port", "value": port, "min": 1, "max": 65535,
		})
	}
	return nil
}
