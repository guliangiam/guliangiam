package captchaproviders

import (
	"strings"

	domaincaptchaproviders "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateCreateCaptchaProviderCommand(command CreateCaptchaProviderCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	// providerType is a closed set (手册 4.14.3): an unknown value used to be
	// stored verbatim and only failed later, when the captcha was generated.
	if err := domaincaptchaproviders.ProviderTypes.ValidateRequired("providerType", command.ProviderType); err != nil {
		return err
	}

	return nil
}

func validatePatchCaptchaProviderCommand(command PatchCaptchaProviderCommand) error {
	if command.Name != nil && strings.TrimSpace(*command.Name) == "" {
		return apperrors.NewInvalidArgument("name must not be blank")
	}
	if err := domaincaptchaproviders.ProviderTypes.ValidateOptionalRequired("providerType", command.ProviderType); err != nil {
		return err
	}

	return nil
}
