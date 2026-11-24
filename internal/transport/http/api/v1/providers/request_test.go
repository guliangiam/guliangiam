package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateProviderRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := CreateProviderRequest{
			OrganizationID: "org-001",
			Name:           "google-oidc",
			DisplayName:    "Google OIDC",
			Category:       "OIDC",
			ProviderType:   "google",
			ClientID:       "client-123",
			ClientSecret:   "secret-456",
			ProviderURL:    "https://accounts.google.com",
			AgentID:        "1000002",
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "google-oidc", cmd.Name)
		assert.Equal(t, "OIDC", cmd.Category)
		assert.Equal(t, "google", cmd.ProviderType)
		assert.Equal(t, "client-123", cmd.ClientID)
		assert.Equal(t, "https://accounts.google.com", cmd.ProviderURL)
		assert.Equal(t, "1000002", cmd.AgentID)
	})
}

func TestPatchProviderRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		displayName := "Updated Provider"
		category := "SAML"
		agentID := "1000002"
		req := PatchProviderRequest{
			DisplayName: &displayName,
			Category:    &category,
			AgentID:     &agentID,
			Revision:    2,
		}

		cmd := req.toCommand("prov-001")

		assert.Equal(t, "prov-001", cmd.ID)
		assert.Equal(t, &displayName, cmd.DisplayName)
		assert.Equal(t, &category, cmd.Category)
		assert.Equal(t, &agentID, cmd.AgentID)
		assert.Equal(t, int64(2), cmd.Revision)
	})
}

func TestUpdateProviderConfigRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := UpdateProviderConfigRequest{
			ConfigType: "oidc",
			Config:     map[string]any{"issuer": "https://idp.example.com"},
		}

		cmd := req.toCommand("prov-001")

		assert.Equal(t, "prov-001", cmd.ProviderID)
		assert.Equal(t, "oidc", cmd.ConfigType)
		assert.Equal(t, "https://idp.example.com", cmd.Config["issuer"])
	})
}
