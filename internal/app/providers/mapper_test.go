package providers

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/stretchr/testify/assert"
)

func TestToListItem(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		provider := &domain.Provider{
			ID:             "prov-001",
			OrganizationID: "org-001",
			Name:           "google-oidc",
			DisplayName:    "Google OIDC",
			Category:       "OIDC",
			ProviderType:   "google",
			SubType:        "oauth2",
			ClientID:       "client-123",
			ProviderURL:    "https://accounts.google.com",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toListItem(provider)

		assert.Equal(t, "prov-001", result.ID)
		assert.Equal(t, "google-oidc", result.Name)
		assert.Equal(t, "OIDC", result.Category)
		assert.Equal(t, "google", result.ProviderType)
	})
}

func TestToDetail(t *testing.T) {
	t.Run("wecom_agent_id_round_trip", func(t *testing.T) {
		now := time.Now()
		provider := &domain.Provider{
			ID:             "prov-wecom",
			OrganizationID: "org-001",
			Name:           "wecom-internal",
			Category:       "OAuth",
			ProviderType:   "WeCom",
			ClientID:       "ww1234567890abcd",
			AgentID:        "1000002",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toDetail(provider)

		assert.Equal(t, "1000002", result.AgentID)
	})

	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		provider := &domain.Provider{
			ID:             "prov-001",
			OrganizationID: "org-001",
			Name:           "google-oidc",
			DisplayName:    "Google OIDC",
			Category:       "OIDC",
			ProviderType:   "google",
			ClientSecret:   "secret-456",
			Scopes:         []string{"openid", "profile"},
			Revision:       1,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toDetail(provider)

		assert.Equal(t, "prov-001", result.ID)
		assert.True(t, result.ClientSecretSet, "ClientSecret should be marked as set")
		assert.NotEmpty(t, result.ClientSecretMasked, "ClientSecretMasked should be populated")
		assert.NotEqual(t, "secret-456", result.ClientSecretMasked, "mask must not equal the plaintext")
		assert.Equal(t, []string{"openid", "profile"}, result.Scopes)
	})
}

func TestToConfigDetail(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		config := &domain.ProviderConfig{
			ProviderID: "prov-001",
			ConfigType: "oidc",
			Config:     map[string]any{"issuer": "https://idp.example.com"},
			UpdatedAt:  now,
		}

		result := toConfigDetail(config)

		assert.Equal(t, "prov-001", result.ProviderID)
		assert.Equal(t, "oidc", result.ConfigType)
	})
}
