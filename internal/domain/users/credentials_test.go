package users

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// stubAccessSecretVerifier maps plaintexts to their expected hashes.
type stubAccessSecretVerifier struct {
	hashes map[string]string // plain -> hash
}

func (s stubAccessSecretVerifier) IsPasswordCorrect(plainPwd, hashedPwd, _salt string) bool {
	return s.hashes[plainPwd] == hashedPwd
}

func TestUserCredentials_VerifyAccessSecret(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	verifier := stubAccessSecretVerifier{hashes: map[string]string{
		"new-secret": "HASH-NEW",
		"old-secret": "HASH-OLD",
	}}

	graceOpen := now.Add(10 * time.Minute)
	graceExpired := now.Add(-1 * time.Minute)

	tests := []struct {
		name   string
		cred   UserCredentials
		secret string
		want   bool
	}{
		{
			name:   "current hash matches",
			cred:   UserCredentials{AccessSecretHash: "HASH-NEW"},
			secret: "new-secret",
			want:   true,
		},
		{
			name:   "prev hash within grace period",
			cred:   UserCredentials{AccessSecretHash: "HASH-NEW", AccessSecretHashPrev: "HASH-OLD", AccessSecretPrevExpiresAt: &graceOpen},
			secret: "old-secret",
			want:   true,
		},
		{
			name:   "prev hash after grace period fails closed",
			cred:   UserCredentials{AccessSecretHash: "HASH-NEW", AccessSecretHashPrev: "HASH-OLD", AccessSecretPrevExpiresAt: &graceExpired},
			secret: "old-secret",
			want:   false,
		},
		{
			name:   "prev hash without expiry fails closed",
			cred:   UserCredentials{AccessSecretHash: "HASH-NEW", AccessSecretHashPrev: "HASH-OLD"},
			secret: "old-secret",
			want:   false,
		},
		{
			name:   "wrong secret rejected",
			cred:   UserCredentials{AccessSecretHash: "HASH-NEW"},
			secret: "unknown",
			want:   false,
		},
		{
			name:   "empty secret rejected",
			cred:   UserCredentials{AccessSecretHash: "HASH-NEW"},
			secret: "",
			want:   false,
		},
		{
			name:   "no hashes configured rejects everything",
			cred:   UserCredentials{},
			secret: "new-secret",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cred.VerifyAccessSecret(verifier, tt.secret, now)
			assert.Equal(t, tt.want, got)
		})
	}
}
