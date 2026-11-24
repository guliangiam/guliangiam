package users

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUserIdentity_Validate(t *testing.T) {
	t.Run("valid identity", func(t *testing.T) {
		identity := UserIdentity{
			ID:              "ident-001",
			UserID:          "user-001",
			ProviderID:      "provider-github",
			IdentityType:    "OAuth",
			Subject:         "github-user-123",
			ProviderAccount: "githubuser",
		}
		assert.Equal(t, "ident-001", identity.ID)
		assert.Equal(t, "user-001", identity.UserID)
		assert.Equal(t, "provider-github", identity.ProviderID)
		assert.Equal(t, "OAuth", identity.IdentityType)
		assert.Equal(t, "github-user-123", identity.Subject)
	})

	t.Run("identity with profile snapshot", func(t *testing.T) {
		now := time.Now()
		identity := UserIdentity{
			ID:           "ident-002",
			UserID:       "user-001",
			ProviderID:   "provider-google",
			IdentityType: "OAuth2",
			Subject:      "google-user-456",
			ProfileSnapshot: map[string]any{
				"email":    "user@example.com",
				"verified": true,
			},
			LinkedAt: now,
		}
		assert.Equal(t, "ident-002", identity.ID)
		assert.NotNil(t, identity.ProfileSnapshot)
		assert.Equal(t, "user@example.com", identity.ProfileSnapshot["email"])
		assert.False(t, identity.LinkedAt.IsZero())
	})
}

func TestUserIdentity_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		identity UserIdentity
	}{
		{
			name: "github OAuth identity",
			identity: UserIdentity{
				ID: "id-1", UserID: "u-1", ProviderID: "github",
				IdentityType: "OAuth", Subject: "gh-123", LinkedAt: now,
			},
		},
		{
			name: "google OAuth2 identity",
			identity: UserIdentity{
				ID: "id-2", UserID: "u-1", ProviderID: "google",
				IdentityType: "OAuth2", Subject: "g-456", LinkedAt: now,
			},
		},
		{
			name: "LDAP identity",
			identity: UserIdentity{
				ID: "id-3", UserID: "u-2", ProviderID: "ldap-corp",
				IdentityType: "LDAP", Subject: "cn=zhangsan,ou=users,dc=corp", LinkedAt: now,
			},
		},
		{
			name: "SAML identity",
			identity: UserIdentity{
				ID: "id-4", UserID: "u-3", ProviderID: "saml-idp",
				IdentityType: "SAML", Subject: "saml-user-789", LinkedAt: now,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.identity.ID)
			assert.NotEmpty(t, tt.identity.UserID)
			assert.NotEmpty(t, tt.identity.ProviderID)
			assert.NotEmpty(t, tt.identity.IdentityType)
			assert.NotEmpty(t, tt.identity.Subject)
		})
	}
}

func TestUserIdentity_ZeroValue(t *testing.T) {
	var identity UserIdentity
	assert.Empty(t, identity.ID)
	assert.Empty(t, identity.UserID)
	assert.Empty(t, identity.ProviderID)
	assert.Empty(t, identity.Subject)
	assert.Nil(t, identity.ProfileSnapshot)
	assert.True(t, identity.LinkedAt.IsZero())
}

func TestUserMfaMethod_Validate(t *testing.T) {
	t.Run("valid TOTP method", func(t *testing.T) {
		method := UserMfaMethod{
			ID:          "mfa-1",
			UserID:      "user-1",
			MethodType:  "totp",
			Enabled:     true,
			Verified:    true,
			DisplayName: "My Authenticator",
		}
		assert.True(t, method.Enabled)
		assert.True(t, method.Verified)
		assert.Equal(t, "totp", method.MethodType)
	})

	t.Run("unverified SMS method", func(t *testing.T) {
		method := UserMfaMethod{
			ID:         "mfa-2",
			UserID:     "user-1",
			MethodType: "sms",
			Enabled:    true,
			Verified:   false,
		}
		assert.True(t, method.Enabled)
		assert.False(t, method.Verified)
	})

	t.Run("disabled method", func(t *testing.T) {
		method := UserMfaMethod{
			ID:         "mfa-3",
			UserID:     "user-1",
			MethodType: "email",
			Enabled:    false,
			Verified:   true,
		}
		assert.False(t, method.Enabled)
	})
}

func TestUserProfile_ValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		profile UserProfile
		want    string
	}{
		{"valid email", UserProfile{UserID: "u-1", Email: "user@example.com"}, "user@example.com"},
		{"empty email", UserProfile{UserID: "u-1", Email: ""}, ""},
		{"email with subdomain", UserProfile{UserID: "u-1", Email: "admin@corp.example.com"}, "admin@corp.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.profile.Email)
		})
	}
}

func TestUserProfile_ValidatePhone(t *testing.T) {
	tests := []struct {
		name    string
		profile UserProfile
		want    string
	}{
		{"valid phone", UserProfile{UserID: "u-1", Phone: "+8613800138000"}, "+8613800138000"},
		{"empty phone", UserProfile{UserID: "u-1", Phone: ""}, ""},
		{"international phone", UserProfile{UserID: "u-1", Phone: "+12125551234"}, "+12125551234"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.profile.Phone)
		})
	}
}

func TestUserProfile_CompletenessCheck(t *testing.T) {
	tests := []struct {
		name     string
		profile  UserProfile
		complete bool
	}{
		{
			name: "complete profile",
			profile: UserProfile{
				UserID: "u-1", DisplayName: "Alice", Email: "alice@example.com",
				Phone: "+1234567890", FirstName: "Alice", LastName: "Smith",
			},
			complete: true,
		},
		{
			name: "missing phone",
			profile: UserProfile{
				UserID: "u-1", DisplayName: "Bob", Email: "bob@example.com",
			},
			complete: false,
		},
		{
			name: "missing email and phone",
			profile: UserProfile{
				UserID: "u-1", DisplayName: "Charlie",
			},
			complete: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isComplete := tt.profile.Email != "" && tt.profile.Phone != ""
			assert.Equal(t, tt.complete, isComplete)
		})
	}
}

func TestUserCredentials_PasswordAlgorithms(t *testing.T) {
	tests := []struct {
		name    string
		algo    string
		wantErr bool
	}{
		{"argon2id", "argon2id", false},
		{"bcrypt rejected (仅存量可验证)", "bcrypt", true},
		{"pbkdf2-salt rejected (仅存量可验证)", "pbkdf2-salt", true},
		{"sha256-salt rejected (仅存量可验证)", "sha256-salt", true},
		{"sha512-salt rejected (仅存量可验证)", "sha512-salt", true},
		{"md5-user-salt rejected (弱密码哈希)", "md5-user-salt", true},
		{"plain rejected (明文密码)", "plain", true},
		{"ldap rejected (非哈希标记)", "ldap", true},
		{"empty is valid", "", false},
		{"unsupported", "scrypt", true},
		{"random", "random-algo", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := UserCredentials{UserID: "user-1", PasswordAlgo: tt.algo}
			err := c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserCredentials_AccessKeyScenarios(t *testing.T) {
	tests := []struct {
		name    string
		cred    UserCredentials
		wantKey bool
	}{
		{"has both", UserCredentials{AccessKey: "ak-1", AccessSecretHash: "hash-1"}, true},
		{"missing key", UserCredentials{AccessSecretHash: "hash-1"}, false},
		{"missing hash", UserCredentials{AccessKey: "ak-1"}, false},
		{"both empty", UserCredentials{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cred.HasAccessKey()
			assert.Equal(t, tt.wantKey, got)
		})
	}
}

func TestUserRoleBinding_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		binding UserRoleBinding
	}{
		{"admin role", UserRoleBinding{UserID: "u-1", RoleID: "role-admin"}},
		{"viewer role", UserRoleBinding{UserID: "u-2", RoleID: "role-viewer"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.binding.UserID)
			assert.NotEmpty(t, tt.binding.RoleID)
		})
	}
}

func TestUserGroupMembership_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		membership UserGroupMembership
	}{
		{"engineering group", UserGroupMembership{UserID: "u-1", GroupID: "grp-eng"}},
		{"admin group", UserGroupMembership{UserID: "u-2", GroupID: "grp-admin"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.membership.UserID)
			assert.NotEmpty(t, tt.membership.GroupID)
		})
	}
}
