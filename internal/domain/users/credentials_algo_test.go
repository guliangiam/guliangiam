package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserCredentials_PasswordVerificationAlgorithm(t *testing.T) {
	tests := []struct {
		name           string
		algo           string
		hash           string
		wantAlgo       string
		wantVerifiable bool
	}{
		{"argon2id", PasswordAlgorithmArgon2ID, "$argon2id$hash", PasswordAlgorithmArgon2ID, true},
		{"empty algo defaults to argon2id", "", "$argon2id$hash", PasswordAlgorithmArgon2ID, true},
		{"empty hash has no local password", PasswordAlgorithmArgon2ID, "", "", false},
		{"ldap marker is not a hash format", "ldap", "some-hash", "", false},
		{"plain is not a local hash format", "plain", "some-hash", "", false},
		{"md5 is not a local hash format", "md5-salt", "some-hash", "", false},
		{"legacy bcrypt stays verifiable read-only", "bcrypt", "some-hash", "bcrypt", true},
		{"legacy pbkdf2 stays verifiable read-only", "pbkdf2-salt", "some-hash", "pbkdf2-salt", true},
		{"legacy sha256 stays verifiable read-only", "sha256-salt", "some-hash", "sha256-salt", true},
		{"legacy sha512 stays verifiable read-only", "sha512-salt", "some-hash", "sha512-salt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentials := &UserCredentials{PasswordAlgo: tt.algo, PasswordHash: tt.hash}
			algo, ok := credentials.PasswordVerificationAlgorithm()
			assert.Equal(t, tt.wantVerifiable, ok)
			assert.Equal(t, tt.wantAlgo, algo)
		})
	}

	t.Run("nil receiver", func(t *testing.T) {
		var credentials *UserCredentials
		algo, ok := credentials.PasswordVerificationAlgorithm()
		assert.False(t, ok)
		assert.Empty(t, algo)
	})
}

func TestPasswordHistoryEntry_PasswordVerificationAlgorithm(t *testing.T) {
	tests := []struct {
		name   string
		algo   string
		hash   string
		wantOK bool
	}{
		{"argon2id history", PasswordAlgorithmArgon2ID, "hash", true},
		{"empty algo defaults to argon2id", "", "hash", true},
		{"empty hash", PasswordAlgorithmArgon2ID, "", false},
		{"ldap marker", "ldap", "hash", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &PasswordHistoryEntry{PasswordAlgo: tt.algo, PasswordHash: tt.hash}
			_, ok := entry.PasswordVerificationAlgorithm()
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}
