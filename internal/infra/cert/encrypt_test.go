package cert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----"
	encrypted, err := EncryptPrivateKey(plaintext, key)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	decrypted, err := DecryptPrivateKey(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptPrivateKey_InvalidKeySize(t *testing.T) {
	_, err := EncryptPrivateKey("test", []byte("short"))
	assert.Error(t, err)

	_, err = EncryptPrivateKey("test", make([]byte, 16))
	assert.Error(t, err)

	_, err = EncryptPrivateKey("test", make([]byte, 64))
	assert.Error(t, err)
}

func TestDecryptPrivateKey_InvalidKeySize(t *testing.T) {
	_, err := DecryptPrivateKey("test", []byte("short"))
	assert.Error(t, err)
}

func TestDecryptPrivateKey_InvalidBase64(t *testing.T) {
	key := make([]byte, 32)
	_, err := DecryptPrivateKey("!!!invalid-base64!!!", key)
	assert.Error(t, err)
}

func TestDecryptPrivateKey_CiphertextTooShort(t *testing.T) {
	key := make([]byte, 32)
	encoded := "AQID"
	_, err := DecryptPrivateKey(encoded, key)
	assert.Error(t, err)
}

func TestEncryptPrivateKey_EmptyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	encrypted, err := EncryptPrivateKey("", key)
	require.NoError(t, err)

	decrypted, err := DecryptPrivateKey(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, "", decrypted)
}

func TestEncryptPrivateKey_DifferentKeys(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1

	encrypted, err := EncryptPrivateKey("secret", key1)
	require.NoError(t, err)

	_, err = DecryptPrivateKey(encrypted, key2)
	assert.Error(t, err)
}

func TestEncryptPrivateKey_ProducesDifferentCiphertexts(t *testing.T) {
	key := make([]byte, 32)
	enc1, err := EncryptPrivateKey("same-plaintext", key)
	require.NoError(t, err)
	enc2, err := EncryptPrivateKey("same-plaintext", key)
	require.NoError(t, err)

	assert.NotEqual(t, enc1, enc2, "different nonces should produce different ciphertexts")
}

func TestNewACMERenewer(t *testing.T) {
	renewer := NewACMERenewer("test@example.com", false)
	assert.NotNil(t, renewer)
	assert.Equal(t, "https://acme-v02.api.letsencrypt.org/directory", renewer.directoryURL)
	assert.False(t, renewer.isStaging)
}

func TestNewACMERenewer_Staging(t *testing.T) {
	renewer := NewACMERenewer("test@example.com", true)
	assert.NotNil(t, renewer)
	assert.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", renewer.directoryURL)
	assert.True(t, renewer.isStaging)
}

func TestACMEEnabled_Default(t *testing.T) {
	result := ACMEEnabled()
	assert.False(t, result)
}

func TestACMEDefaultRenewBefore(t *testing.T) {
	d := ACMEDefaultRenewBefore()
	assert.Equal(t, 30*24*time.Hour, d)
}

func TestACMEDefaultCheckInterval(t *testing.T) {
	d := ACMEDefaultCheckInterval()
	assert.Equal(t, 24*time.Hour, d)
}
