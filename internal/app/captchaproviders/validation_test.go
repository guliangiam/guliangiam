package captchaproviders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCreateCaptchaProviderCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateCaptchaProviderCommand{OrganizationID: "org-001", Name: "recaptcha", ProviderType: "recaptcha"}
		assert.NoError(t, validateCreateCaptchaProviderCommand(cmd))
	})
	t.Run("missing_org", func(t *testing.T) {
		cmd := CreateCaptchaProviderCommand{Name: "recaptcha", ProviderType: "recaptcha"}
		assert.Error(t, validateCreateCaptchaProviderCommand(cmd))
	})
	t.Run("missing_name", func(t *testing.T) {
		cmd := CreateCaptchaProviderCommand{OrganizationID: "org-001", ProviderType: "recaptcha"}
		assert.Error(t, validateCreateCaptchaProviderCommand(cmd))
	})
	t.Run("missing_type", func(t *testing.T) {
		cmd := CreateCaptchaProviderCommand{OrganizationID: "org-001", Name: "recaptcha"}
		assert.Error(t, validateCreateCaptchaProviderCommand(cmd))
	})
}

func TestValidatePatchCaptchaProviderCommand(t *testing.T) {
	t.Run("valid_nil", func(t *testing.T) {
		assert.NoError(t, validatePatchCaptchaProviderCommand(PatchCaptchaProviderCommand{}))
	})
	t.Run("blank_name", func(t *testing.T) {
		blank := ""
		assert.Error(t, validatePatchCaptchaProviderCommand(PatchCaptchaProviderCommand{Name: &blank}))
	})
	t.Run("blank_type", func(t *testing.T) {
		blank := ""
		assert.Error(t, validatePatchCaptchaProviderCommand(PatchCaptchaProviderCommand{ProviderType: &blank}))
	})
}
