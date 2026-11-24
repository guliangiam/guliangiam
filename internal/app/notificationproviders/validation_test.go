package notificationproviders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCreateNotificationProviderCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateNotificationProviderCommand{OrganizationID: "org-001", Name: "ses", ProviderType: "email"}
		assert.NoError(t, validateCreateNotificationProviderCommand(cmd))
	})
	t.Run("missing_org", func(t *testing.T) {
		cmd := CreateNotificationProviderCommand{Name: "ses", ProviderType: "email"}
		assert.Error(t, validateCreateNotificationProviderCommand(cmd))
	})
	t.Run("missing_name", func(t *testing.T) {
		cmd := CreateNotificationProviderCommand{OrganizationID: "org-001", ProviderType: "email"}
		assert.Error(t, validateCreateNotificationProviderCommand(cmd))
	})
	t.Run("missing_type", func(t *testing.T) {
		cmd := CreateNotificationProviderCommand{OrganizationID: "org-001", Name: "ses"}
		assert.Error(t, validateCreateNotificationProviderCommand(cmd))
	})
}

func TestValidatePatchNotificationProviderCommand(t *testing.T) {
	t.Run("valid_nil", func(t *testing.T) {
		assert.NoError(t, validatePatchNotificationProviderCommand(PatchNotificationProviderCommand{}))
	})
	t.Run("blank_name", func(t *testing.T) {
		blank := ""
		assert.Error(t, validatePatchNotificationProviderCommand(PatchNotificationProviderCommand{Name: &blank}))
	})
	t.Run("blank_type", func(t *testing.T) {
		blank := ""
		assert.Error(t, validatePatchNotificationProviderCommand(PatchNotificationProviderCommand{ProviderType: &blank}))
	})
}
