package radiusconfigs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
)

// sealRadiusSecret 用生产 mapper 生成密文，保证断言的是真实 envelope 令牌。
func sealRadiusSecret(t *testing.T, configID, plaintext string) string {
	t.Helper()
	sealed, err := radiusConfigEncryptedMapper.EncryptColumn(radiusConfigAAD(configID), plaintext)
	require.NoError(t, err)
	return sealed
}

func TestToRadiusConfig(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		row := radiusConfigRow{
			ID:             "rc-001",
			OrganizationID: "org-001",
			Name:           "main-radius",
			Host:           "radius.example.com",
			Port:           1812,
			SharedSecret:   sealRadiusSecret(t, "rc-001", "secret-abc"),
			AuthProtocol:   "PAP",
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		result, err := toRadiusConfig(row)
		require.NoError(t, err)

		assert.Equal(t, "rc-001", result.ID)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "main-radius", result.Name)
		assert.Equal(t, "radius.example.com", result.Host)
		assert.Equal(t, 1812, result.Port)
		assert.Equal(t, "secret-abc", result.SharedSecret)
		assert.Equal(t, "PAP", result.AuthProtocol)
		assert.True(t, result.IsEnabled)
		assert.Equal(t, int64(1), result.Revision)
	})

	t.Run("minimal_fields", func(t *testing.T) {
		now := time.Now()
		row := radiusConfigRow{
			ID:             "rc-002",
			OrganizationID: "org-002",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result, err := toRadiusConfig(row)
		require.NoError(t, err)

		assert.Equal(t, "rc-002", result.ID)
		assert.Equal(t, 0, result.Port)
		assert.Empty(t, result.SharedSecret)
		assert.False(t, result.IsEnabled)
	})

	t.Run("plaintext_secret_fails_closed", func(t *testing.T) {
		row := radiusConfigRow{ID: "rc-003", SharedSecret: "legacy-plaintext"}

		_, err := toRadiusConfig(row)
		require.Error(t, err, "未回填的明文行不得当作可用明文返回")
	})
}

func TestRadiusSecretForWrite(t *testing.T) {
	config := &domain.RadiusConfig{ID: "rc-004", SharedSecret: "secret-to-seal"}

	sealed, err := radiusSecretForWrite(config)
	require.NoError(t, err)
	require.NotEqual(t, config.SharedSecret, sealed)
	require.Equal(t, "secret-to-seal", config.SharedSecret, "加密不得修改 domain 对象")

	opened, err := radiusConfigEncryptedMapper.DecryptColumn(radiusConfigAAD(config.ID), sealed)
	require.NoError(t, err)
	require.Equal(t, "secret-to-seal", opened)
}
