package cred

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHashProvider_AllAlgorithms(t *testing.T) {
	tests := []struct {
		algo      string
		wantAlgo  string
		wantError bool
	}{
		{AlgoArgon2id, AlgoArgon2id, false},
		{AlgoBcrypt, AlgoBcrypt, false},
		{AlgoPBKDF2, AlgoPBKDF2, false},
		{AlgoSHA256, AlgoSHA256, false},
		{AlgoSHA512, AlgoSHA512, false},
		{AlgoMD5, AlgoMD5, false},
		{AlgoPlain, AlgoPlain, false},
		{"unknown", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.algo, func(t *testing.T) {
			provider, err := NewHashProvider(tt.algo)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantAlgo, provider.Algorithm())
			}
		})
	}
}

func TestNewHashProvider_RoundTrip(t *testing.T) {
	algos := []string{AlgoArgon2id, AlgoBcrypt, AlgoPBKDF2, AlgoSHA256, AlgoSHA512, AlgoMD5, AlgoPlain}
	password := "test-password-123"

	for _, algo := range algos {
		t.Run(algo, func(t *testing.T) {
			provider, err := NewHashProvider(algo)
			require.NoError(t, err)

			hash, err := provider.Hash(password)
			require.NoError(t, err)
			assert.NotEmpty(t, hash)

			ok, err := provider.Verify(password, hash)
			require.NoError(t, err)
			assert.True(t, ok, "password should verify against its own hash")

			ok, err = provider.Verify("wrong-password", hash)
			require.NoError(t, err)
			assert.False(t, ok, "wrong password should not verify")
		})
	}
}

func TestDetectAlgorithm(t *testing.T) {
	tests := []struct {
		name string
		hash string
		want string
	}{
		{"argon2id", "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$hash", AlgoArgon2id},
		{"bcrypt $2a$", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", AlgoBcrypt},
		{"bcrypt $2b$", "$2b$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", AlgoBcrypt},
		{"bcrypt $2y$", "$2y$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", AlgoBcrypt},
		{"pbkdf2", "pbkdf2-sha256$i=600000$l=c2FsdA$a2V5", AlgoPBKDF2},
		{"sha256", "{SHA256}abc123$def456", AlgoSHA256},
		{"sha512", "{SHA512}abc123$def456", AlgoSHA512},
		{"md5_32chars", "5d41402abc4b2a76b9719d911017c592", AlgoMD5},
		{"unknown_fallback", "some-random-string", AlgoArgon2id},
		{"empty_fallback", "", AlgoArgon2id},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectAlgorithm(tt.hash)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectAlgorithm_ThenVerify(t *testing.T) {
	password := "round-trip-detect"

	// Plain text has no identifiable prefix, so DetectAlgorithm cannot detect it
	for _, algo := range []string{AlgoArgon2id, AlgoBcrypt, AlgoSHA256, AlgoSHA512, AlgoMD5} {
		t.Run(algo, func(t *testing.T) {
			provider, err := NewHashProvider(algo)
			require.NoError(t, err)

			hash, err := provider.Hash(password)
			require.NoError(t, err)

			detected := DetectAlgorithm(hash)
			assert.Equal(t, algo, detected, "DetectAlgorithm should identify the correct algorithm from its own hash")
		})
	}
}
