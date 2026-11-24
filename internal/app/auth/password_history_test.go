package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePasswordHistoryHash(t *testing.T) {
	hash1, err := createPasswordHistoryHash("password123")
	require.NoError(t, err)

	hash2, err := createPasswordHistoryHash("password123")
	require.NoError(t, err)

	// argon2id uses random salt, so same password produces different hashes.
	assert.NotEqual(t, hash1, hash2, "argon2id with random salt should produce different hashes for same password")

	hash3, err := createPasswordHistoryHash("different")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash3, "different passwords should produce different hashes")
}

func TestCreatePasswordHistoryHash_Argon2idFormat(t *testing.T) {
	hash, err := createPasswordHistoryHash("my-secret-password")
	require.NoError(t, err)

	// argon2id encoded hash should start with "$argon2id$"
	assert.True(t, hash != "" && hash[:10] == "$argon2id$", "hash should have argon2id prefix")
	assert.NotContains(t, hash, "my-secret-password", "hash should not contain plaintext password")
}

func TestVerifyPasswordHistoryHash_Argon2id(t *testing.T) {
	password := "test-password-argon2id"

	hash, err := createPasswordHistoryHash(password)
	require.NoError(t, err)

	matched, err := verifyPasswordHistoryHash(password, hash)
	require.NoError(t, err)
	assert.True(t, matched, "password should match its argon2id hash")

	matched, err = verifyPasswordHistoryHash("wrong-password", hash)
	require.NoError(t, err)
	assert.False(t, matched, "wrong password should not match")
}

func TestVerifyPasswordHistoryHash_LegacySHA256(t *testing.T) {
	password := "test-password-sha256"

	// Generate a legacy SHA-256 hash using the deprecated function.
	legacyHash := hashPasswordForHistorySHA256(password)

	// Verify that the new verify function can still check legacy hashes.
	matched, err := verifyPasswordHistoryHash(password, legacyHash)
	require.NoError(t, err)
	assert.True(t, matched, "password should match its legacy SHA-256 hash")

	matched, err = verifyPasswordHistoryHash("wrong-password", legacyHash)
	require.NoError(t, err)
	assert.False(t, matched, "wrong password should not match legacy hash")
}

func TestVerifyPasswordHistoryHash_LegacySHA256Format(t *testing.T) {
	// Legacy SHA-256 hashes should be 64-char hex strings.
	legacyHash := hashPasswordForHistorySHA256("some-password")
	assert.Len(t, legacyHash, 64, "SHA-256 hex should be 64 chars")
	assert.False(t, legacyHash != "" && legacyHash[:10] == "$argon2id$", "legacy hash should not have argon2id prefix")
}
