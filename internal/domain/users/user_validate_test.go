package users

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestUser_Validate(t *testing.T) {
	t.Run("valid_user", func(t *testing.T) {
		u := User{ID: "user-001", OrganizationID: "org-001", Username: "testuser"}
		assert.NoError(t, u.Validate())
	})

	t.Run("missing_id", func(t *testing.T) {
		u := User{OrganizationID: "org-001", Username: "testuser"}
		assert.Error(t, u.Validate())
	})

	t.Run("missing_organizationId", func(t *testing.T) {
		u := User{ID: "user-001", Username: "testuser"}
		assert.Error(t, u.Validate())
	})

	t.Run("missing_username", func(t *testing.T) {
		u := User{ID: "user-001", OrganizationID: "org-001"}
		assert.Error(t, u.Validate())
	})
}

func TestUser_SoftDeleted(t *testing.T) {
	t.Run("not_deleted", func(t *testing.T) {
		u := User{ID: "user-001"}
		assert.False(t, u.SoftDeleted())
	})

	t.Run("soft_deleted", func(t *testing.T) {
		now := time.Now()
		u := User{ID: "user-001", DeletedAt: &now}
		assert.True(t, u.SoftDeleted())
	})
}

func TestUser_CanLogin(t *testing.T) {
	t.Run("normal_user_can_login", func(t *testing.T) {
		u := User{IsForbidden: false}
		assert.True(t, u.CanLogin())
	})

	t.Run("forbidden_user_cannot_login", func(t *testing.T) {
		u := User{IsForbidden: true}
		assert.False(t, u.CanLogin())
	})

	t.Run("deleted_user_cannot_login", func(t *testing.T) {
		now := time.Now()
		u := User{DeletedAt: &now}
		assert.False(t, u.CanLogin())
	})

	t.Run("forbidden_and_deleted_cannot_login", func(t *testing.T) {
		now := time.Now()
		u := User{IsForbidden: true, DeletedAt: &now}
		assert.False(t, u.CanLogin())
	})
}

func TestUser_IsVerifiedIdentified(t *testing.T) {
	t.Run("not_verified", func(t *testing.T) {
		u := User{IsVerified: false}
		assert.False(t, u.IsVerifiedIdentified())
	})

	t.Run("verified", func(t *testing.T) {
		u := User{IsVerified: true}
		assert.True(t, u.IsVerifiedIdentified())
	})
}

func TestUserCredentials_Validate(t *testing.T) {
	t.Run("valid_credentials", func(t *testing.T) {
		c := UserCredentials{UserID: "user-001", PasswordAlgo: "argon2id"}
		assert.NoError(t, c.Validate())
	})

	t.Run("missing_userId", func(t *testing.T) {
		c := UserCredentials{PasswordAlgo: "argon2id"}
		assert.Error(t, c.Validate())
	})

	t.Run("unsupported_algorithm", func(t *testing.T) {
		c := UserCredentials{UserID: "user-001", PasswordAlgo: "unknown-algo"}
		assert.Error(t, c.Validate())
	})

	t.Run("only_argon2id_is_writable", func(t *testing.T) {
		algorithms := []string{"argon2id"}
		for _, algo := range algorithms {
			c := UserCredentials{UserID: "user-001", PasswordAlgo: algo}
			assert.NoError(t, c.Validate(), "algorithm %s should be valid", algo)
		}
	})

	t.Run("legacy_and_weak_algorithms_rejected", func(t *testing.T) {
		algorithms := []string{
			"bcrypt", "pbkdf2-salt", "sha256-salt", "sha512-salt",
			"md5-user-salt", "plain", "ldap",
		}
		for _, algo := range algorithms {
			c := UserCredentials{UserID: "user-001", PasswordAlgo: algo}
			assert.Error(t, c.Validate(), "algorithm %s should be rejected", algo)
		}
	})

	t.Run("empty_algorithm_is_valid", func(t *testing.T) {
		c := UserCredentials{UserID: "user-001", PasswordAlgo: ""}
		assert.NoError(t, c.Validate())
	})
}

func TestUserCredentials_IsPasswordExpired(t *testing.T) {
	now := time.Now()

	t.Run("not_expired", func(t *testing.T) {
		updatedAt := now.Add(-30 * 24 * time.Hour) // 30 days ago
		c := UserCredentials{PasswordUpdatedAt: &updatedAt}
		assert.False(t, c.IsPasswordExpired(now, 90)) // 90-day policy
	})

	t.Run("expired", func(t *testing.T) {
		updatedAt := now.Add(-100 * 24 * time.Hour) // 100 days ago
		c := UserCredentials{PasswordUpdatedAt: &updatedAt}
		assert.True(t, c.IsPasswordExpired(now, 90)) // 90-day policy
	})

	t.Run("nil_updatedAt_not_expired", func(t *testing.T) {
		c := UserCredentials{PasswordUpdatedAt: nil}
		assert.False(t, c.IsPasswordExpired(now, 90))
	})

	t.Run("zero_maxDays_not_expired", func(t *testing.T) {
		updatedAt := now.Add(-1000 * 24 * time.Hour)
		c := UserCredentials{PasswordUpdatedAt: &updatedAt}
		assert.False(t, c.IsPasswordExpired(now, 0))
	})
}

func TestUserCredentials_HasAccessKey(t *testing.T) {
	t.Run("has_key", func(t *testing.T) {
		c := UserCredentials{AccessKey: "ak-123", AccessSecretHash: "hash-123"}
		assert.True(t, c.HasAccessKey())
	})

	t.Run("missing_key", func(t *testing.T) {
		c := UserCredentials{AccessSecretHash: "hash-123"}
		assert.False(t, c.HasAccessKey())
	})

	t.Run("missing_hash", func(t *testing.T) {
		c := UserCredentials{AccessKey: "ak-123"}
		assert.False(t, c.HasAccessKey())
	})
}

func TestUserIdentity_Fields(t *testing.T) {
	identity := UserIdentity{
		ID:              "ident-001",
		UserID:          "user-001",
		ProviderID:      "provider-github",
		IdentityType:    "OAuth",
		Subject:         "github-user-123",
		ProviderAccount: "githubuser",
	}

	assert.Equal(t, "ident-001", identity.ID)
	assert.Equal(t, "provider-github", identity.ProviderID)
	assert.Equal(t, "OAuth", identity.IdentityType)
	assert.Equal(t, "github-user-123", identity.Subject)
}

func TestUserMfaMethod_Fields(t *testing.T) {
	now := time.Now()
	method := UserMfaMethod{
		ID:          "mfa-001",
		UserID:      "user-001",
		MethodType:  "totp",
		Enabled:     true,
		Verified:    true,
		DisplayName: "My Authenticator",
		LastUsedAt:  &now,
	}

	assert.Equal(t, "mfa-001", method.ID)
	assert.True(t, method.Enabled)
	assert.True(t, method.Verified)
	assert.Equal(t, "totp", method.MethodType)
}

func TestUserProfile_ValidateTest(t *testing.T) {
	profile := UserProfile{
		UserID:        "user-001",
		DisplayName:   "张三",
		FirstName:     "三",
		LastName:      "张",
		Email:         "zhangsan@example.com",
		EmailVerified: true,
		Phone:         "+8613800138000",
		PhoneVerified: false,
		RealName:      "张三",
		IDCardType:    "idcard",
		IDCard:        "110101199001011234",
	}

	assert.Equal(t, "user-001", profile.UserID)
	assert.True(t, profile.EmailVerified)
	assert.False(t, profile.PhoneVerified)
	assert.NotEmpty(t, profile.RealName)
}

func TestUserCredentials_Validate_AlgorithmAnswers422(t *testing.T) {
	credentials := &UserCredentials{UserID: "user-1", PasswordAlgo: "md5"}
	appErr := apperrors.FromError(credentials.Validate())
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "algorithm", appErr.Meta["field"])
	assert.NotNil(t, appErr.Meta["allowed"])
}
