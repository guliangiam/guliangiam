package providers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func encryptTestSecret(t *testing.T, plaintext string) string {
	t.Helper()
	sealed, err := crypto.GlobalFieldEncryptor().EncryptField(plaintext)
	require.NoError(t, err)
	return sealed
}

func TestToProvider(t *testing.T) {
	now := time.Now()

	t.Run("oauth_provider", func(t *testing.T) {
		row := providerRow{
			ID:             "prov-001",
			OrganizationID: "org-001",
			Name:           "github",
			DisplayName:    "GitHub",
			Category:       "OAuth",
			ProviderType:   "GitHub",
			ClientID:       "gh-client-id",
			ClientSecret:   encryptTestSecret(t, "gh-client-secret"),
			AuthUrl:        "https://github.com/login/oauth/authorize",
			TokenUrl:       "https://github.com/login/oauth/access_token",
			UserInfoUrl:    "https://api.github.com/user",
			Scopes:         []string{"user:email", "read:org"},
			UserMapping:    []byte(`{"email":"email","name":"login"}`),
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		provider, err := toProvider(row)
		require.NoError(t, err)

		assert.Equal(t, "prov-001", provider.ID)
		assert.Equal(t, "org-001", provider.OrganizationID)
		assert.Equal(t, "github", provider.Name)
		assert.Equal(t, "GitHub", provider.DisplayName)
		assert.Equal(t, "OAuth", string(provider.Category))
		assert.Equal(t, "GitHub", string(provider.ProviderType))
		assert.Equal(t, "gh-client-id", provider.ClientID)
		assert.Equal(t, "gh-client-secret", provider.ClientSecret)
		assert.Equal(t, []string{"user:email", "read:org"}, provider.Scopes)
		assert.Equal(t, int64(1), provider.Revision)
	})

	t.Run("saml_provider", func(t *testing.T) {
		row := providerRow{
			ID:             "prov-002",
			OrganizationID: "org-001",
			Name:           "okta-saml",
			DisplayName:    "Okta SAML",
			Category:       "SAML",
			ProviderType:   "Okta",
			IdPEntityId:    "https://example.okta.com",
			SsoUrl:         "https://example.okta.com/app/sso",
			SloUrl:         "https://example.okta.com/app/slo",
			Metadata:       "<md:EntityDescriptor>...</md:EntityDescriptor>",
			SigningAlg:     "RSA-SHA256",
			UserMapping:    []byte(`{}`),
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		provider, err := toProvider(row)
		require.NoError(t, err)

		assert.Equal(t, "SAML", string(provider.Category))
		assert.Equal(t, "https://example.okta.com", provider.IdPEntityId)
		assert.Equal(t, "RSA-SHA256", provider.SigningAlg)
	})

	t.Run("email_notification_provider", func(t *testing.T) {
		row := providerRow{
			ID:             "prov-003",
			OrganizationID: "org-001",
			Name:           "smtp",
			DisplayName:    "SMTP Server",
			Category:       "Notification",
			ProviderType:   "SMTP",
			Host:           "smtp.example.com",
			Port:           587,
			Admin:          "noreply@example.com",
			Password:       encryptTestSecret(t, "smtp-password"),
			UserMapping:    nil,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		provider, err := toProvider(row)
		require.NoError(t, err)

		assert.Equal(t, "Notification", string(provider.Category))
		assert.Equal(t, "smtp.example.com", provider.Host)
		assert.Equal(t, 587, provider.Port)
		assert.Equal(t, "smtp-password", provider.Password)
	})

	t.Run("storage_provider", func(t *testing.T) {
		row := providerRow{
			ID:             "prov-004",
			OrganizationID: "org-001",
			Name:           "s3",
			DisplayName:    "AWS S3",
			Category:       "Storage",
			ProviderType:   "S3",
			Region:         "us-east-1",
			ClientID:       "AKIAIOSFODNN7EXAMPLE",
			ClientSecret:   encryptTestSecret(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
			UserMapping:    nil,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		provider, err := toProvider(row)
		require.NoError(t, err)
		assert.Equal(t, "Storage", string(provider.Category))
		assert.Equal(t, "us-east-1", provider.Region)
		assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", provider.ClientSecret)
	})
}

func TestToProvider_RejectsPlaintextSecret(t *testing.T) {
	row := providerRow{
		ID:           "prov-plain",
		ClientSecret: "plain-client-secret",
		UserMapping:  []byte(`{}`),
	}

	_, err := toProvider(row)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not envelope-encrypted")
}
