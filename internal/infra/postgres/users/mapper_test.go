package users

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToUser(t *testing.T) {
	now := time.Now()

	t.Run("basic_mapping", func(t *testing.T) {
		row := userRow{
			ID:              "user-001",
			OrganizationID:  "org-001",
			Username:        "testuser",
			UserType:        "normal",
			Status:          "active",
			IsAdmin:         false,
			IsForbidden:     false,
			IsVerified:      true,
			IsOnline:        true,
			CreatedIp:       "127.0.0.1",
			RegisterType:    "password",
			RegisterSource:  "web",
			Tag:             "vip",
			Score:           100,
			Balance:         50.0,
			BalanceCredit:   10.0,
			BalanceCurrency: "CNY",
			IsDeleted:       false,
			IpWhitelist:     "",
			Custom1:         "c1",
			Custom2:         "c2",
			CreatedAt:       now,
			UpdatedAt:       now,
			Revision:        1,
		}

		user, err := toUser(row)
		require.NoError(t, err)

		assert.Equal(t, "user-001", user.ID)
		assert.Equal(t, "org-001", user.OrganizationID)
		assert.Equal(t, "testuser", user.Username)
		assert.Equal(t, "normal", user.UserType)
		assert.Equal(t, "active", user.Status)
		assert.False(t, user.IsAdmin)
		assert.True(t, user.IsVerified)
		assert.True(t, user.IsOnline)
		assert.Equal(t, "127.0.0.1", user.CreatedIp)
		assert.Equal(t, "password", user.RegisterType)
		assert.Equal(t, "vip", user.Tag)
		assert.Equal(t, 100, user.Score)
		assert.Equal(t, 50.0, user.Balance)
		assert.Equal(t, "CNY", user.BalanceCurrency)
		assert.False(t, user.IsDeleted)
		assert.Equal(t, int64(1), user.Revision)
	})

	t.Run("with_jsonb_fields", func(t *testing.T) {
		row := userRow{
			ID:                 "user-002",
			OrganizationID:     "org-001",
			Username:           "jsonuser",
			RecoveryCodeHashes: []string{"code1", "code2"},
			MfaItems:           []byte(`[{"type":"totp","enabled":true}]`),
			ApplicationScopes:  []byte(`[]`),
			CreatedAt:          now,
			UpdatedAt:          now,
			Revision:           1,
		}

		user, err := toUser(row)
		require.NoError(t, err)

		assert.Equal(t, "user-002", user.ID)
		assert.NotNil(t, user.RecoveryCodeHashes)
		assert.NotNil(t, user.MfaItems)
		assert.NotNil(t, user.ApplicationScopes)
	})

	t.Run("with_null_jsonb_fields", func(t *testing.T) {
		row := userRow{
			ID:                 "user-003",
			OrganizationID:     "org-001",
			Username:           "nulluser",
			RecoveryCodeHashes: nil,
			Addresses:          nil,
			ManagedAccounts:    nil,
			FaceIds:            nil,
			Cart:               nil,
			MfaItems:           nil,
			MfaAccounts:        nil,
			ApplicationScopes:  nil,
			CreatedAt:          now,
			UpdatedAt:          now,
			Revision:           1,
		}

		user, err := toUser(row)
		require.NoError(t, err)
		assert.Equal(t, "user-003", user.ID)
	})

	t.Run("with_soft_delete", func(t *testing.T) {
		deletedAt := now.Add(-24 * time.Hour)
		row := userRow{
			ID:             "user-004",
			OrganizationID: "org-001",
			Username:       "deleteduser",
			IsDeleted:      true,
			DeletedAt:      &deletedAt,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       2,
		}

		user, err := toUser(row)
		require.NoError(t, err)
		assert.True(t, user.IsDeleted)
		assert.NotNil(t, user.DeletedAt)
	})

	t.Run("admin_user", func(t *testing.T) {
		row := userRow{
			ID:             "admin-001",
			OrganizationID: "org-001",
			Username:       "admin",
			IsAdmin:        true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		user, err := toUser(row)
		require.NoError(t, err)
		assert.True(t, user.IsAdmin)
	})
}

func TestApplyProfile(t *testing.T) {
	t.Run("basic_profile", func(t *testing.T) {
		target := &domain.User{ID: "user-001"}
		row := userProfileRow{
			UserID:      "user-001",
			DisplayName: "Test User",
			FirstName:   "Test",
			LastName:    "User",
			AvatarURL:   "https://example.com/avatar.png",
			Email:       "test@example.com",
			Phone:       "+8613800138000",
			CountryCode: "CN",
			Region:      "Beijing",
			Gender:      "male",
		}

		err := applyProfile(target, row)
		require.NoError(t, err)

		assert.Equal(t, "user-001", target.Profile.UserID)
		assert.Equal(t, "Test User", target.Profile.DisplayName)
		assert.Equal(t, "test@example.com", target.Profile.Email)
		assert.Equal(t, "+8613800138000", target.Profile.Phone)
	})

	t.Run("with_real_name_verification", func(t *testing.T) {
		target := &domain.User{ID: "user-002"}
		row := userProfileRow{
			UserID:     "user-002",
			RealName:   "Zhang San",
			IDCardType: "idcard",
			IDCard:     "110101199001011234",
		}

		err := applyProfile(target, row)
		require.NoError(t, err)
		assert.Equal(t, "Zhang San", target.Profile.RealName)
		assert.Equal(t, "idcard", target.Profile.IDCardType)
	})

	t.Run("with_properties_json", func(t *testing.T) {
		target := &domain.User{ID: "user-003"}
		row := userProfileRow{
			UserID:     "user-003",
			Properties: []byte(`{"theme":"dark","lang":"zh-CN"}`),
		}

		err := applyProfile(target, row)
		require.NoError(t, err)
		assert.NotNil(t, target.Profile.Properties)
	})

	t.Run("with_affiliation_in_properties", func(t *testing.T) {
		target := &domain.User{ID: "user-004"}
		row := userProfileRow{
			UserID:     "user-004",
			Properties: []byte(`{"affiliation":"Beijing University","theme":"dark"}`),
		}

		err := applyProfile(target, row)
		require.NoError(t, err)
		assert.Equal(t, "Beijing University", target.Profile.Affiliation)
	})

	t.Run("sealed_document_numbers_preferred", func(t *testing.T) {
		target := &domain.User{ID: "user-005"}
		idSealed, err := sealProfileDocument(userProfileColumnIDCard, "user-005", "110101199001011234")
		require.NoError(t, err)
		passportSealed, err := sealProfileDocument(userProfileColumnPassportNo, "user-005", "E12345678")
		require.NoError(t, err)
		driverSealed, err := sealProfileDocument(userProfileColumnDriverLicenseNo, "user-005", "D1234567")
		require.NoError(t, err)
		row := userProfileRow{
			UserID: "user-005",
			// 双写窗口内明文列可能滞后：sealed 孪生列优先。
			IDCard:                "stale-plaintext",
			IDCardSealed:          idSealed,
			PassportNoSealed:      passportSealed,
			DriverLicenseNoSealed: driverSealed,
		}

		require.NoError(t, applyProfile(target, row))
		assert.Equal(t, "110101199001011234", target.Profile.IDCard)
		assert.Equal(t, "E12345678", target.Profile.PassportNo)
		assert.Equal(t, "D1234567", target.Profile.DriverLicenseNo)
	})

	t.Run("legacy_plaintext_fallback_when_sealed_null", func(t *testing.T) {
		target := &domain.User{ID: "user-006"}
		row := userProfileRow{UserID: "user-006", IDCard: "110101199001011234"}

		require.NoError(t, applyProfile(target, row))
		assert.Equal(t, "110101199001011234", target.Profile.IDCard)
	})

	t.Run("unreadable_sealed_fails_closed", func(t *testing.T) {
		target := &domain.User{ID: "user-007"}
		forged := "not-an-envelope"
		row := userProfileRow{UserID: "user-007", IDCard: "110101199001011234", IDCardSealed: &forged}

		require.Error(t, applyProfile(target, row))
		assert.Empty(t, target.Profile.IDCard)
	})
}

func TestApplyCredentials(t *testing.T) {
	now := time.Now()

	t.Run("with_password", func(t *testing.T) {
		target := &domain.User{ID: "user-001"}
		row := userCredentialsRow{
			UserID:            "user-001",
			PasswordHash:      "$argon2id$v=19$m=65536,t=3,p=4$hash",
			PasswordAlgo:      "argon2id",
			PasswordUpdatedAt: &now,
			AccessKey:         "AK-001",
			AccessSecretHash:  "sha256hash",
		}

		applyCredentials(target, row)

		assert.True(t, target.Credentials.HasPassword)
		assert.Equal(t, "argon2id", target.Credentials.PasswordAlgo)
		assert.Equal(t, "AK-001", target.Credentials.AccessKey)
		assert.Equal(t, "sha256hash", target.Credentials.AccessSecretHash)
	})

	t.Run("no_password", func(t *testing.T) {
		target := &domain.User{ID: "user-002"}
		row := userCredentialsRow{
			UserID:       "user-002",
			PasswordHash: "",
			PasswordAlgo: "",
		}

		applyCredentials(target, row)
		assert.False(t, target.Credentials.HasPassword)
	})

	t.Run("need_update_password", func(t *testing.T) {
		target := &domain.User{ID: "user-003"}
		row := userCredentialsRow{
			UserID:             "user-003",
			PasswordHash:       "somehash",
			NeedUpdatePassword: true,
		}

		applyCredentials(target, row)
		assert.True(t, target.Credentials.NeedUpdatePassword)
	})
}

func TestToIdentity(t *testing.T) {
	now := time.Now()

	t.Run("basic_identity", func(t *testing.T) {
		row := userIdentityRow{
			ID:              "idn-001",
			UserID:          "user-001",
			ProviderID:      "github",
			IdentityType:    "oauth",
			Subject:         "12345",
			ProviderAccount: "ghuser",
			LinkedAt:        now,
		}

		identity, err := toIdentity(row)
		require.NoError(t, err)

		assert.Equal(t, "idn-001", identity.ID)
		assert.Equal(t, "user-001", identity.UserID)
		assert.Equal(t, "github", identity.ProviderID)
		assert.Equal(t, "oauth", identity.IdentityType)
		assert.Equal(t, "12345", identity.Subject)
	})

	t.Run("with_profile_snapshot", func(t *testing.T) {
		row := userIdentityRow{
			ID:              "idn-002",
			UserID:          "user-001",
			ProviderID:      "google",
			IdentityType:    "oauth",
			Subject:         "67890",
			ProfileSnapshot: []byte(`{"email":"user@gmail.com","name":"Test"}`),
			LinkedAt:        now,
		}

		identity, err := toIdentity(row)
		require.NoError(t, err)
		assert.NotNil(t, identity.ProfileSnapshot)
	})
}

func TestToMfaMethod(t *testing.T) {
	lastUsed := time.Now()

	t.Run("enabled_method", func(t *testing.T) {
		row := userMfaMethodRow{
			ID:          "mfa-001",
			UserID:      "user-001",
			MethodType:  "totp",
			Enabled:     true,
			Verified:    true,
			DisplayName: "Authenticator",
			LastUsedAt:  &lastUsed,
		}

		method := toMfaMethod(row)
		assert.Equal(t, "mfa-001", method.ID)
		assert.Equal(t, "totp", method.MethodType)
		assert.True(t, method.Enabled)
		assert.True(t, method.Verified)
	})

	t.Run("disabled_method", func(t *testing.T) {
		row := userMfaMethodRow{
			ID:         "mfa-002",
			UserID:     "user-001",
			MethodType: "sms",
			Enabled:    false,
			Verified:   false,
		}

		method := toMfaMethod(row)
		assert.False(t, method.Enabled)
		assert.False(t, method.Verified)
	})
}
