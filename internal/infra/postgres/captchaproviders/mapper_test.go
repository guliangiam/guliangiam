package captchaproviders

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
)

// sealCaptchaSecret 用生产 mapper 生成密文，保证断言的是真实 envelope 令牌。
func sealCaptchaSecret(t *testing.T, providerID, plaintext string) string {
	t.Helper()
	sealed, err := captchaProviderEncryptedMapper.EncryptColumn(captchaProviderAAD(providerID), plaintext)
	require.NoError(t, err)
	return sealed
}

func TestToCaptchaProvider(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		config := map[string]any{"timeout": 30, "theme": "light"}
		configJSON, err := json.Marshal(config)
		require.NoError(t, err)

		row := captchaProviderRow{
			ID:             "cp-001",
			OrganizationID: "org-001",
			Name:           "recaptcha-v2",
			ProviderType:   "google",
			SiteKey:        "site-key-123",
			SecretKey:      sealCaptchaSecret(t, "cp-001", "secret-key-456"),
			Config:         configJSON,
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		result, err := toCaptchaProvider(row)
		require.NoError(t, err)

		assert.Equal(t, "cp-001", result.ID)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "recaptcha-v2", result.Name)
		assert.Equal(t, "google", result.ProviderType)
		assert.Equal(t, "site-key-123", result.SiteKey)
		assert.Equal(t, "secret-key-456", result.SecretKey, "读取时应解密回明文")
		assert.True(t, result.IsEnabled)
		assert.Equal(t, float64(30), result.Config["timeout"])
	})

	t.Run("nil_config_becomes_empty_map", func(t *testing.T) {
		now := time.Now()
		row := captchaProviderRow{
			ID:             "cp-002",
			OrganizationID: "org-001",
			Name:           "hcaptcha",
			Config:         nil,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result, err := toCaptchaProvider(row)
		require.NoError(t, err)

		assert.Equal(t, map[string]any{}, result.Config)
	})

	t.Run("invalid_config_json_returns_error", func(t *testing.T) {
		row := captchaProviderRow{
			ID:     "cp-003",
			Config: []byte(`{invalid}`),
		}

		_, err := toCaptchaProvider(row)
		assert.Error(t, err)
	})

	t.Run("plaintext_secret_fails_closed", func(t *testing.T) {
		row := captchaProviderRow{ID: "cp-004", SecretKey: "legacy-plaintext"}

		_, err := toCaptchaProvider(row)
		require.Error(t, err, "未回填的明文行不得当作可用明文返回")
	})
}

func TestCaptchaSecretForWrite(t *testing.T) {
	provider := &domain.CaptchaProvider{ID: "cp-005", SecretKey: "secret-to-seal"}

	sealed, err := captchaSecretForWrite(provider)
	require.NoError(t, err)
	require.NotEqual(t, provider.SecretKey, sealed)
	require.Equal(t, "secret-to-seal", provider.SecretKey, "加密不得修改 domain 对象")

	opened, err := captchaProviderEncryptedMapper.DecryptColumn(captchaProviderAAD(provider.ID), sealed)
	require.NoError(t, err)
	require.Equal(t, "secret-to-seal", opened)
}
