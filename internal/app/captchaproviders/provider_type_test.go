package captchaproviders

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaincaptchaproviders "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestValidateCreateCaptchaProviderCommand_RejectsUnknownProviderType(t *testing.T) {
	// Every spelling the console used to send. All six must be rejected: the
	// factory only knows the lowercase keys.
	rejected := []string{
		"Default", "reCAPTCHA", "hCaptcha", "Turnstile", "GeeTest", "Aliyun",
		"google", "GoogleRecaptcha", "HCaptcha", "Recaptcha", "RECAPTCHA",
		"", "  ", "unsupported-provider",
	}
	for _, providerType := range rejected {
		t.Run("reject_"+providerType, func(t *testing.T) {
			cmd := CreateCaptchaProviderCommand{
				OrganizationID: "org-001",
				Name:           "provider",
				ProviderType:   providerType,
			}
			err := validateCreateCaptchaProviderCommand(cmd)
			require.Error(t, err, "providerType %q must not be accepted", providerType)
		})
	}
}

func TestValidateCreateCaptchaProviderCommand_UnknownTypeIs422WithAllowedValues(t *testing.T) {
	cmd := CreateCaptchaProviderCommand{
		OrganizationID: "org-001",
		Name:           "provider",
		ProviderType:   "Turnstile",
	}

	err := validateCreateCaptchaProviderCommand(cmd)
	require.Error(t, err)

	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code,
		"a well-formed request naming an unknown value is unprocessable, not malformed")
	require.NotNil(t, appErr.Meta)
	assert.Equal(t, "providerType", appErr.Meta["field"])
	assert.Equal(t, "Turnstile", appErr.Meta["value"])
	allowed, ok := appErr.Meta["allowed"].([]string)
	require.True(t, ok, "allowed must carry the set so the caller can fix the request")
	assert.ElementsMatch(t, domaincaptchaproviders.ProviderTypes.Values(), allowed)
}

// A blank value is an absent argument, not an unknown one: the two must stay
// distinguishable so a client is not told to retry a request that cannot
// succeed without a change.
func TestValidateCreateCaptchaProviderCommand_BlankTypeIs400(t *testing.T) {
	cmd := CreateCaptchaProviderCommand{OrganizationID: "org-001", Name: "provider"}

	err := validateCreateCaptchaProviderCommand(cmd)
	require.Error(t, err)

	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestValidatePatchCaptchaProviderCommand_RejectsUnknownButAllowsAbsent(t *testing.T) {
	unknown := "Turnstile"
	err := validatePatchCaptchaProviderCommand(PatchCaptchaProviderCommand{ProviderType: &unknown})
	require.Error(t, err)

	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)

	ok := "hcaptcha"
	assert.NoError(t, validatePatchCaptchaProviderCommand(PatchCaptchaProviderCommand{ProviderType: &ok}))
}

func TestValidateCreateCaptchaProviderCommand_AcceptsEveryDeclaredType(t *testing.T) {
	for _, providerType := range domaincaptchaproviders.ProviderTypes.Values() {
		t.Run(providerType, func(t *testing.T) {
			cmd := CreateCaptchaProviderCommand{
				OrganizationID: "org-001",
				Name:           "provider",
				ProviderType:   providerType,
			}
			assert.NoError(t, validateCreateCaptchaProviderCommand(cmd))
		})
	}
}
