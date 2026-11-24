package notificationproviders

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func TestToNotificationProvider(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		config := map[string]any{"region": "us-east-1", "sender": "noreply@example.com"}
		configJSON, err := json.Marshal(config)
		require.NoError(t, err)
		sealed, err := crypto.GlobalFieldEncryptor().EncryptField("token-abc")
		require.NoError(t, err)

		row := notificationProviderRow{
			ID:             "np-001",
			OrganizationID: "org-001",
			Name:           "ses-provider",
			ProviderType:   "email",
			WebhookURL:     "https://hooks.example.com",
			AccessToken:    sealed,
			Config:         configJSON,
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		result, err := toNotificationProvider(row)
		require.NoError(t, err)

		assert.Equal(t, "np-001", result.ID)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "ses-provider", result.Name)
		assert.Equal(t, "email", result.ProviderType)
		assert.Equal(t, "https://hooks.example.com", result.WebhookURL)
		assert.Equal(t, "token-abc", result.AccessToken)
		assert.True(t, result.IsEnabled)
		assert.Equal(t, "us-east-1", result.Config["region"])
	})

	t.Run("nil_config_becomes_empty_map", func(t *testing.T) {
		now := time.Now()
		row := notificationProviderRow{
			ID:             "np-002",
			OrganizationID: "org-001",
			Name:           "smtp-provider",
			Config:         nil,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result, err := toNotificationProvider(row)
		require.NoError(t, err)

		assert.Equal(t, map[string]any{}, result.Config)
	})

	t.Run("invalid_config_json_returns_error", func(t *testing.T) {
		row := notificationProviderRow{
			ID:     "np-003",
			Config: []byte(`{invalid}`),
		}

		_, err := toNotificationProvider(row)
		assert.Error(t, err)
	})

	t.Run("plaintext_access_token_fails_closed", func(t *testing.T) {
		row := notificationProviderRow{ID: "np-004", AccessToken: "legacy-plaintext"}

		_, err := toNotificationProvider(row)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not envelope-encrypted")
		assert.Contains(t, err.Error(), "encrypt-provider-secrets")
		assert.NotContains(t, err.Error(), "legacy-plaintext")
	})
}
