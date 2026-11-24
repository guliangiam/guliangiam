package notificationproviders

import (
	"strings"

	domainnotificationproviders "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateCreateNotificationProviderCommand(command CreateNotificationProviderCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	// providerType is a closed set (手册 4.14.3): an unknown value used to be
	// stored verbatim and only failed later, when the notification was sent.
	if err := domainnotificationproviders.ProviderTypes.ValidateRequired("providerType", command.ProviderType); err != nil {
		return err
	}

	return nil
}

func validatePatchNotificationProviderCommand(command PatchNotificationProviderCommand) error {
	if command.Name != nil && strings.TrimSpace(*command.Name) == "" {
		return apperrors.NewInvalidArgument("name must not be blank")
	}
	if err := domainnotificationproviders.ProviderTypes.ValidateOptionalRequired("providerType", command.ProviderType); err != nil {
		return err
	}

	return nil
}
