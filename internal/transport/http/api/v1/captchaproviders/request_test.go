package captchaproviders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateCaptchaProviderRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		isEnabled := true
		req := CreateCaptchaProviderRequest{
			OrganizationID: "org-001",
			Name:           "recaptcha",
			ProviderType:   "google",
			SiteKey:        "site-key",
			SecretKey:      "secret-key",
			Config:         map[string]any{"theme": "dark"},
			IsEnabled:      &isEnabled,
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "recaptcha", cmd.Name)
		assert.Equal(t, "google", cmd.ProviderType)
		assert.Equal(t, "dark", cmd.Config["theme"])
	})
}

func TestPatchCaptchaProviderRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		name := "updated-recaptcha"
		req := PatchCaptchaProviderRequest{
			Name:     &name,
			Revision: 2,
		}

		cmd := req.toCommand("cp-001")

		assert.Equal(t, "cp-001", cmd.ID)
		assert.Equal(t, &name, cmd.Name)
	})
}
