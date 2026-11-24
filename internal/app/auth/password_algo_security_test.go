package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/cred"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func legacyBcryptHash(t *testing.T, password string) string {
	t.Helper()
	manager := cred.GetCredManager("bcrypt")
	require.NotNil(t, manager)
	hash := manager.GetHashedPassword(password, "")
	require.NotEmpty(t, hash)
	return hash
}

// GM-PWD-1a: even when the old password was stored with a historical
// algorithm, the new password is hashed with argon2id and the old algorithm
// value is never written back.
func TestChangePassword_WritesArgon2IDForLegacyHash(t *testing.T) {
	org := newTestOrg()
	legacyHash := legacyBcryptHash(t, testPassword)
	user := newTestUser(func(u *userdomain.User) {
		u.Credentials.PasswordHash = legacyHash
		u.Credentials.PasswordAlgo = "bcrypt"
	})
	uow := newTestUow(org, user, newTestApp())
	uow.tokenRepo = &testTokenRepository{}
	svc := newTestService(uow)

	const newPassword = "Xq7!mK2#vR9z"
	_, _, err := svc.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      testPassword,
		NewPassword:      newPassword,
	})
	require.NoError(t, err)

	require.NotEmpty(t, uow.userRepo.updatedCredentials)
	updated := uow.userRepo.updatedCredentials[len(uow.userRepo.updatedCredentials)-1]
	assert.Equal(t, userdomain.PasswordAlgorithmArgon2ID, updated.PasswordAlgo)
	assert.True(t, updated.HasPassword)
	assert.NotEqual(t, legacyHash, updated.PasswordHash, "legacy hash must not be carried over")
	assert.True(t,
		cred.GetCredManager(userdomain.PasswordAlgorithmArgon2ID).
			IsPasswordCorrect(newPassword, updated.PasswordHash, ""),
		"new hash must verify with argon2id",
	)
}

// 改密成功后必须清除 need_update_password，否则前端会一直提示改密。
func TestChangePassword_ClearsNeedUpdatePassword(t *testing.T) {
	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) {
		u.Credentials.NeedUpdatePassword = true
	})
	uow := newTestUow(org, user, newTestApp())
	uow.tokenRepo = &testTokenRepository{}
	svc := newTestService(uow)

	_, _, err := svc.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      testPassword,
		NewPassword:      "Xq7!mK2#vR9z",
	})
	require.NoError(t, err)

	require.NotEmpty(t, uow.userRepo.updatedCredentials)
	updated := uow.userRepo.updatedCredentials[len(uow.userRepo.updatedCredentials)-1]
	assert.False(t, updated.NeedUpdatePassword, "need_update_password must be cleared after a self-service change")
}

// GM-PWD-1a: the login compatibility path may verify a legacy hash, but it
// must immediately rewrite the credential as argon2id.
func TestLogin_UpgradesLegacyHashToArgon2ID(t *testing.T) {
	org := newTestOrg()
	legacyHash := legacyBcryptHash(t, testPassword)
	user := newTestUser(func(u *userdomain.User) {
		u.Credentials.PasswordHash = legacyHash
		u.Credentials.PasswordAlgo = "bcrypt"
	})
	uow := newTestUow(org, user, newTestApp())
	svc := newTestService(uow)

	_, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	require.NoError(t, err)

	require.NotEmpty(t, uow.userRepo.updatedCredentials)
	upgraded := uow.userRepo.updatedCredentials[len(uow.userRepo.updatedCredentials)-1]
	assert.Equal(t, userdomain.PasswordAlgorithmArgon2ID, upgraded.PasswordAlgo)
	assert.NotEqual(t, legacyHash, upgraded.PasswordHash)
	assert.True(t,
		cred.GetCredManager(userdomain.PasswordAlgorithmArgon2ID).
			IsPasswordCorrect(testPassword, upgraded.PasswordHash, ""),
		"upgraded hash must verify with argon2id",
	)
}

// GM-PWD-1a: no-local-password and provider-marker rows must never select a
// verifier. Legacy bcrypt remains verifiable read-only for the upgrade path.
func TestVerifyPassword_RejectsNonLocalCredentials(t *testing.T) {
	legacyHash := legacyBcryptHash(t, testPassword)

	tests := []struct {
		name     string
		password string
		algo     string
		hash     string
		wantErr  bool
	}{
		{"empty hash is not a password", testPassword, userdomain.PasswordAlgorithmArgon2ID, "", true},
		{"ldap marker with hash is not a password format", testPassword, "ldap", testPasswordHash, true},
		{"plain hash is rejected", testPassword, "plain", testPassword, true},
		{"unknown marker is rejected", testPassword, "g38a-no-such-credential-manager", testPasswordHash, true},
		{"wrong argon2id password is rejected", "WrongPassword!", userdomain.PasswordAlgorithmArgon2ID, testPasswordHash, true},
		{"empty algo defaults to argon2id", testPassword, "", testPasswordHash, false},
		{"legacy bcrypt remains verifiable for upgrade", testPassword, "bcrypt", legacyHash, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyPassword(tt.password, tt.algo, tt.hash)
			if tt.wantErr {
				require.Error(t, err)
				typed, ok := err.(*apperrors.Error)
				require.True(t, ok)
				assert.Equal(t, apperrors.CodeUnauthorized, typed.Code)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCheckPassword_NoLocalPasswordIsInvalid(t *testing.T) {
	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) {
		u.Credentials.HasPassword = false
		u.Credentials.PasswordHash = ""
		u.Credentials.PasswordAlgo = "ldap"
	})
	uow := newTestUow(org, user, newTestApp())
	svc := newTestService(uow)

	result, err := svc.CheckPassword(context.Background(), CheckPasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Valid)
}
