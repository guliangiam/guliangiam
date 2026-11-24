package notificationproviders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateNotificationProviderRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		isEnabled := true
		req := CreateNotificationProviderRequest{
			OrganizationID: "org-001",
			Name:           "ses-provider",
			ProviderType:   "email",
			WebhookURL:     "https://hooks.example.com",
			AccessToken:    "token-abc",
			Config:         map[string]any{"region": "us-east-1"},
			IsEnabled:      &isEnabled,
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "ses-provider", cmd.Name)
		assert.Equal(t, "email", cmd.ProviderType)
		assert.Equal(t, "us-east-1", cmd.Config["region"])
	})
}

func TestPatchNotificationProviderRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		name := "updated-provider"
		req := PatchNotificationProviderRequest{
			Name:     &name,
			Revision: 2,
		}

		cmd := req.toCommand("np-001")

		assert.Equal(t, "np-001", cmd.ID)
		assert.Equal(t, &name, cmd.Name)
	})
}
