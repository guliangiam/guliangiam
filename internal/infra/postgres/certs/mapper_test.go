package certs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
)

// sealCertSecrets 用生产 mapper 生成密文，保证断言的是真实 envelope 令牌。
func sealCertSecrets(t *testing.T, certID, privateKey, accessSecret string) (string, string) {
	t.Helper()
	sealedKey, err := certEncryptedMapper.EncryptColumn(certAAD(certID, certColumnPrivateKey), privateKey)
	require.NoError(t, err)
	sealedSecret, err := certEncryptedMapper.EncryptColumn(certAAD(certID, certColumnAccessSecret), accessSecret)
	require.NoError(t, err)
	return sealedKey, sealedSecret
}

func TestToCert(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		expireTime := now.Add(365 * 24 * time.Hour)
		domainExpireTime := now.Add(30 * 24 * time.Hour)
		domainList := []string{"example.com", "api.example.com"}
		domainJSON, err := json.Marshal(domainList)
		require.NoError(t, err)
		sealedKey, sealedSecret := sealCertSecrets(t, "cert-001", "-----BEGIN KEY-----", "secret-456")

		row := certRow{
			ID:                     "cert-001",
			OrganizationID:         "org-001",
			Name:                   "my-cert",
			DisplayName:            "My Certificate",
			Scope:                  "global",
			Type:                   "x509",
			CryptoAlgorithm:        "RSA",
			BitSize:                2048,
			ExpireInYears:          1,
			ExpireTime:             &expireTime,
			Certificate:            "-----BEGIN CERT-----",
			PrivateKey:             sealedKey,
			AuthorityPublicKey:     "pub-key",
			AuthorityRootPublicKey: "root-pub-key",
			AcmeIssuer:             "letsencrypt",
			AcmeEndpoint:           "https://acme-v02.api.letsencrypt.org/directory",
			Provider:               "acme",
			Account:                "account@example.com",
			AccessKey:              "key-123",
			AccessSecret:           sealedSecret,
			Domain:                 domainJSON,
			DomainExpireTime:       &domainExpireTime,
			CreatedAt:              now,
			UpdatedAt:              now,
			Revision:               1,
		}

		result, err := toCert(row)
		require.NoError(t, err)

		assert.Equal(t, "cert-001", result.ID)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "my-cert", result.Name)
		assert.Equal(t, "RSA", result.CryptoAlgorithm)
		assert.Equal(t, 2048, result.BitSize)
		assert.Equal(t, "-----BEGIN KEY-----", result.PrivateKey, "读取时应解密回明文")
		assert.Equal(t, "secret-456", result.AccessSecret, "读取时应解密回明文")
		assert.Equal(t, domainList, result.Domain)
		assert.Equal(t, &expireTime, result.ExpireTime)
	})

	t.Run("nil_domain_becomes_empty_slice", func(t *testing.T) {
		now := time.Now()
		row := certRow{
			ID:             "cert-002",
			OrganizationID: "org-001",
			Name:           "no-domain-cert",
			Domain:         nil,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result, err := toCert(row)
		require.NoError(t, err)

		assert.Equal(t, []string{}, result.Domain)
	})

	t.Run("nil_expire_time", func(t *testing.T) {
		now := time.Now()
		row := certRow{
			ID:             "cert-003",
			OrganizationID: "org-001",
			Name:           "no-expire-cert",
			Domain:         []byte(`[]`),
			ExpireTime:     nil,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result, err := toCert(row)
		require.NoError(t, err)

		assert.Nil(t, result.ExpireTime)
	})

	t.Run("invalid_domain_json_returns_error", func(t *testing.T) {
		row := certRow{
			ID:             "cert-004",
			OrganizationID: "org-001",
			Name:           "bad-cert",
			Domain:         []byte(`{invalid}`),
		}

		_, err := toCert(row)
		assert.Error(t, err)
	})

	t.Run("plaintext_secret_fails_closed", func(t *testing.T) {
		row := certRow{ID: "cert-005", PrivateKey: "legacy-plaintext-key"}

		_, err := toCert(row)
		require.Error(t, err, "未回填的明文行不得当作可用明文返回")
	})
}

func TestCertSecretsForWrite(t *testing.T) {
	cert := &domain.Cert{ID: "cert-006", PrivateKey: "key-to-seal", AccessSecret: "secret-to-seal"}

	sealedKey, sealedSecret, err := certSecretsForWrite(cert)
	require.NoError(t, err)
	require.NotEqual(t, cert.PrivateKey, sealedKey)
	require.NotEqual(t, cert.AccessSecret, sealedSecret)
	require.Equal(t, "key-to-seal", cert.PrivateKey, "加密不得修改 domain 对象")
	require.Equal(t, "secret-to-seal", cert.AccessSecret, "加密不得修改 domain 对象")

	openedKey, err := certEncryptedMapper.DecryptColumn(certAAD(cert.ID, certColumnPrivateKey), sealedKey)
	require.NoError(t, err)
	require.Equal(t, "key-to-seal", openedKey)
	openedSecret, err := certEncryptedMapper.DecryptColumn(certAAD(cert.ID, certColumnAccessSecret), sealedSecret)
	require.NoError(t, err)
	require.Equal(t, "secret-to-seal", openedSecret)
}
