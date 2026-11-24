package keys

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockScanner struct {
	values []interface{}
	err    error
}

func (m *mockScanner) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	for i, d := range dest {
		switch v := m.values[i].(type) {
		case int64:
			if ptr, ok := d.(*int64); ok {
				*ptr = v
			}
		case string:
			if ptr, ok := d.(*string); ok {
				*ptr = v
			}
		case *string:
			if ptr, ok := d.(**string); ok {
				*ptr = v
			}
		case bool:
			if ptr, ok := d.(*bool); ok {
				*ptr = v
			}
		case time.Time:
			if ptr, ok := d.(*time.Time); ok {
				*ptr = v
			}
		case nil:
			// skip
		}
	}
	return nil
}

func TestScanKey(t *testing.T) {
	now := time.Now()
	privateKey := "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----"
	sealedPrivateKey, err := sealKeyPrivateKey("key-001", privateKey)
	require.NoError(t, err)

	t.Run("full_key_with_private", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"key-001", "org-001", "signing-key", "Signing Key",
				"RSA", "public-key-data", &sealedPrivateKey,
				"RS256", nil, // expiresAt
				true, now, now, int64(1),
			},
		}

		key, err := scanKey(sc)
		require.NoError(t, err)

		assert.Equal(t, "key-001", key.ID)
		assert.Equal(t, "org-001", key.OrganizationID)
		assert.Equal(t, "signing-key", key.Name)
		assert.Equal(t, "RSA", key.KeyType)
		assert.Equal(t, "public-key-data", key.PublicKey)
		assert.Equal(t, privateKey, key.PrivateKey, "读取时应解密回明文")
		assert.Equal(t, "RS256", key.Algorithm)
		assert.True(t, key.IsEnabled)
	})

	t.Run("legacy_plaintext_private_key_fails_closed", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"key-legacy", "org-001", "legacy-key", "Legacy Key",
				"RSA", "public-key-data", &privateKey,
				"RS256", nil,
				true, now, now, int64(1),
			},
		}

		key, err := scanKey(sc)
		require.Error(t, err, "未回填的明文私钥不得当作可用私钥返回")
		assert.Nil(t, key)
		assert.Contains(t, err.Error(), "backfill-encrypted-columns")
	})

	t.Run("key_without_private", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"key-002", "org-001", "public-key", "Public Key",
				"EC", "ec-public-key", nil, // nil privateKey
				"ES256", nil,
				true, now, now, int64(1),
			},
		}

		key, err := scanKey(sc)
		require.NoError(t, err)

		assert.Equal(t, "", key.PrivateKey)
	})

	t.Run("empty_private_key_stays_empty", func(t *testing.T) {
		empty := ""
		sc := &mockScanner{
			values: []interface{}{
				"key-003", "org-001", "empty-key", "Empty Key",
				"RSA", "public-key-data", &empty,
				"RS256", nil,
				true, now, now, int64(1),
			},
		}

		key, err := scanKey(sc)
		require.NoError(t, err)
		assert.Empty(t, key.PrivateKey)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanKey(sc)
		assert.Error(t, err)
	})
}
