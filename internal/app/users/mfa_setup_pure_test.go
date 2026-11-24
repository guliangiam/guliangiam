package users

import (
	"context"
	"errors"
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/mfatotp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
)

func ctx() context.Context { return context.Background() }
func timeNow() time.Time   { return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) }

// validTotpCode derives a code with the same library the server verifies with.
func validTotpCode(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, at)
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}
	return code
}

// ---------------------------------------------------------------------------
// Tests for applyUserMfaSetupProfile
// ---------------------------------------------------------------------------

func TestApplyUserMfaSetupProfile_Email_NoExistingEmail(t *testing.T) {
	profile := &userdomain.UserProfile{}
	changed, err := applyUserMfaSetupProfile(profile, "email", "user@example.com", "")
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "user@example.com", profile.Email)
	assert.True(t, profile.EmailVerified)
}

func TestApplyUserMfaSetupProfile_Email_ExistingEmail(t *testing.T) {
	profile := &userdomain.UserProfile{Email: "existing@example.com", EmailVerified: true}
	changed, err := applyUserMfaSetupProfile(profile, "email", "new@example.com", "")
	assert.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, "existing@example.com", profile.Email)
}

func TestApplyUserMfaSetupProfile_Email_ExistingEmailNotVerified(t *testing.T) {
	profile := &userdomain.UserProfile{Email: "existing@example.com", EmailVerified: false}
	changed, err := applyUserMfaSetupProfile(profile, "email", "new@example.com", "")
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "existing@example.com", profile.Email)
	assert.True(t, profile.EmailVerified)
}

func TestApplyUserMfaSetupProfile_Email_MissingDest(t *testing.T) {
	profile := &userdomain.UserProfile{}
	changed, err := applyUserMfaSetupProfile(profile, "email", "", "")
	assert.Error(t, err)
	assert.False(t, changed)
}

func TestApplyUserMfaSetupProfile_SMS_NoExistingPhone(t *testing.T) {
	profile := &userdomain.UserProfile{}
	changed, err := applyUserMfaSetupProfile(profile, "sms", "+1234567890", "1")
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "+1234567890", profile.Phone)
	assert.Equal(t, "1", profile.CountryCode)
}

func TestApplyUserMfaSetupProfile_SMS_ExistingPhone(t *testing.T) {
	profile := &userdomain.UserProfile{Phone: "+1111111111", CountryCode: "1"}
	changed, err := applyUserMfaSetupProfile(profile, "sms", "+2222222222", "2")
	assert.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, "+1111111111", profile.Phone)
}

func TestApplyUserMfaSetupProfile_SMS_MissingDest(t *testing.T) {
	profile := &userdomain.UserProfile{}
	changed, err := applyUserMfaSetupProfile(profile, "sms", "", "1")
	assert.Error(t, err)
	assert.False(t, changed)
}

func TestApplyUserMfaSetupProfile_SMS_MissingCountryCode(t *testing.T) {
	profile := &userdomain.UserProfile{}
	changed, err := applyUserMfaSetupProfile(profile, "sms", "+1234567890", "")
	assert.Error(t, err)
	assert.False(t, changed)
}

func TestApplyUserMfaSetupProfile_App_NoChange(t *testing.T) {
	profile := &userdomain.UserProfile{}
	changed, err := applyUserMfaSetupProfile(profile, "app", "", "")
	assert.NoError(t, err)
	assert.False(t, changed)
}

func TestApplyUserMfaSetupProfile_Radius_NoChange(t *testing.T) {
	profile := &userdomain.UserProfile{}
	changed, err := applyUserMfaSetupProfile(profile, "radius", "radiususer", "")
	assert.NoError(t, err)
	assert.False(t, changed)
}

func TestApplyUserMfaSetupProfile_Push_NoChange(t *testing.T) {
	profile := &userdomain.UserProfile{}
	changed, err := applyUserMfaSetupProfile(profile, "push", "pushreceiver", "")
	assert.NoError(t, err)
	assert.False(t, changed)
}

// ---------------------------------------------------------------------------
// Tests for applyUserMfaSetupState
// ---------------------------------------------------------------------------

func TestApplyUserMfaSetupState_App(t *testing.T) {
	user := &userdomain.User{PreferredMfaType: ""}
	err := applyUserMfaSetupState(user, "app", "totp", "JBSWY3DPEHPK3PXP", "", []string{"recovery1", "recovery2"})
	assert.NoError(t, err)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", user.TotpSecret)
	assert.Equal(t, "totp", user.PreferredMfaType)
	assert.Len(t, user.RecoveryCodeHashes, 2)
	assertRecoveryHashesMatch(t, user.RecoveryCodeHashes, "recovery1", "recovery2")
}

func TestApplyUserMfaSetupState_App_EmptySecret(t *testing.T) {
	user := &userdomain.User{}
	err := applyUserMfaSetupState(user, "app", "totp", "  ", "", nil)
	assert.Error(t, err)
}

func TestApplyUserMfaSetupState_App_ExistingPreferredMfa(t *testing.T) {
	user := &userdomain.User{PreferredMfaType: "sms"}
	err := applyUserMfaSetupState(user, "app", "totp", "secret123", "", nil)
	assert.NoError(t, err)
	assert.Equal(t, "sms", user.PreferredMfaType) // Should not change
}

func TestApplyUserMfaSetupState_Radius(t *testing.T) {
	user := &userdomain.User{}
	err := applyUserMfaSetupState(user, "radius", "radius", "provider1", "radiususer", nil)
	assert.NoError(t, err)
	assert.Equal(t, "radiususer", user.MfaRadiusUsername)
	assert.Equal(t, "provider1", user.MfaRadiusProvider)
}

func TestApplyUserMfaSetupState_Push(t *testing.T) {
	user := &userdomain.User{}
	err := applyUserMfaSetupState(user, "push", "push", "provider1", "receiver1", nil)
	assert.NoError(t, err)
	assert.Equal(t, "receiver1", user.MfaPushReceiver)
	assert.Equal(t, "provider1", user.MfaPushProvider)
}

func TestApplyUserMfaSetupState_Email_NoStateChange(t *testing.T) {
	user := &userdomain.User{}
	err := applyUserMfaSetupState(user, "email", "email", "", "", []string{"code1"})
	assert.NoError(t, err)
	assert.Len(t, user.RecoveryCodeHashes, 1)
	assertRecoveryHashesMatch(t, user.RecoveryCodeHashes, "code1")
}

func TestApplyUserMfaSetupState_RecoveryCodesWithWhitespace(t *testing.T) {
	user := &userdomain.User{}
	err := applyUserMfaSetupState(user, "app", "totp", "secret", "", []string{"  code1  ", "  ", "code2"})
	assert.NoError(t, err)
	assert.Len(t, user.RecoveryCodeHashes, 2)
	assertRecoveryHashesMatch(t, user.RecoveryCodeHashes, "code1", "code2")
	for _, hash := range user.RecoveryCodeHashes {
		assert.NotEmpty(t, hash)
	}
}

// ---------------------------------------------------------------------------
// Tests for upsertUserMfaSetupMethod
// ---------------------------------------------------------------------------

func TestUpsertUserMfaSetupMethod_ReplaceExisting(t *testing.T) {
	svc := Service{UUID: svcFakeUUID{fixedID: "new-method-id"}}
	methods := []userdomain.UserMfaMethod{
		{ID: "m1", UserID: "u1", MethodType: "totp", Enabled: false, Verified: false},
		{ID: "m2", UserID: "u1", MethodType: "sms", Enabled: true, Verified: true},
	}
	result, err := svc.upsertUserMfaSetupMethod(methods, "u1", "totp", nil)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	// The totp method should be updated
	for _, m := range result {
		if m.MethodType == "totp" {
			assert.True(t, m.Enabled)
			assert.True(t, m.Verified)
		}
	}
}

func TestUpsertUserMfaSetupMethod_AppendNew(t *testing.T) {
	svc := Service{UUID: svcFakeUUID{fixedID: "new-method-id"}}
	methods := []userdomain.UserMfaMethod{
		{ID: "m1", UserID: "u1", MethodType: "sms", Enabled: true, Verified: true},
	}
	result, err := svc.upsertUserMfaSetupMethod(methods, "u1", "totp", nil)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "new-method-id", result[1].ID)
	assert.Equal(t, "totp", result[1].MethodType)
	assert.True(t, result[1].Enabled)
	assert.True(t, result[1].Verified)
	assert.Equal(t, "Authenticator App", result[1].DisplayName)
}

func TestUpsertUserMfaSetupMethod_EmptyMethods(t *testing.T) {
	svc := Service{UUID: svcFakeUUID{fixedID: "new-method-id"}}
	result, err := svc.upsertUserMfaSetupMethod(nil, "u1", "email", nil)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "email", result[0].MethodType)
	assert.Equal(t, "Email", result[0].DisplayName)
}

func TestUpsertUserMfaSetupMethod_ExistingWithDisplayName(t *testing.T) {
	svc := Service{UUID: svcFakeUUID{fixedID: "new-method-id"}}
	methods := []userdomain.UserMfaMethod{
		{ID: "m1", UserID: "u1", MethodType: "sms", Enabled: false, Verified: false, DisplayName: "My SMS"},
	}
	result, err := svc.upsertUserMfaSetupMethod(methods, "u1", "sms", nil)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "My SMS", result[0].DisplayName) // Should keep existing display name
}

func TestUpsertUserMfaSetupMethod_UUIDError(t *testing.T) {
	svc := Service{UUID: svcFakeUUIDErr{}}
	_, err := svc.upsertUserMfaSetupMethod(nil, "u1", "totp", nil)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Tests for userMfaRememberHours / parseUserMfaRememberHours
// ---------------------------------------------------------------------------

func TestUserMfaRememberHours_NilOrganization(t *testing.T) {
	hours, ok := userMfaRememberHours(nil)
	assert.Equal(t, int64(0), hours)
	assert.False(t, ok)
}

func TestUserMfaRememberHours_EmptyPolicy(t *testing.T) {
	org := &orgdomain.Organization{}
	hours, ok := userMfaRememberHours(org)
	assert.Equal(t, int64(0), hours)
	assert.False(t, ok)
}

func TestUserMfaRememberHours_WithPolicy(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			SessionPolicy: map[string]any{"mfaRememberHours": 24},
		},
	}
	hours, ok := userMfaRememberHours(org)
	assert.Equal(t, int64(24), hours)
	assert.True(t, ok)
}

func TestParseUserMfaRememberHours_Empty(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(nil)
	assert.Equal(t, int64(0), hours)
	assert.False(t, ok)
}

func TestParseUserMfaRememberHours_Int(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"mfaRememberHours": 48})
	assert.Equal(t, int64(48), hours)
	assert.True(t, ok)
}

func TestParseUserMfaRememberHours_Int32(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"mfaRememberHours": int32(12)})
	assert.Equal(t, int64(12), hours)
	assert.True(t, ok)
}

func TestParseUserMfaRememberHours_Int64(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"mfaRememberHours": int64(72)})
	assert.Equal(t, int64(72), hours)
	assert.True(t, ok)
}

func TestParseUserMfaRememberHours_Float64(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"mfaRememberHours": float64(36)})
	assert.Equal(t, int64(36), hours)
	assert.True(t, ok)
}

func TestParseUserMfaRememberHours_String(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"mfaRememberHours": "48"})
	assert.Equal(t, int64(48), hours)
	assert.True(t, ok)
}

func TestParseUserMfaRememberHours_EmptyString(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"mfaRememberHours": "  "})
	assert.Equal(t, int64(0), hours)
	assert.False(t, ok)
}

func TestParseUserMfaRememberHours_InvalidString(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"mfaRememberHours": "not-a-number"})
	assert.Equal(t, int64(0), hours)
	assert.False(t, ok)
}

func TestParseUserMfaRememberHours_ZeroValue(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"mfaRememberHours": 0})
	assert.Equal(t, int64(0), hours)
	assert.False(t, ok) // 0 is not > 0
}

func TestParseUserMfaRememberHours_NegativeValue(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"mfaRememberHours": -1})
	assert.Equal(t, int64(-1), hours)
	assert.False(t, ok) // -1 is not > 0
}

func TestParseUserMfaRememberHours_AlternateKeyName(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"mfaRememberInHours": 24})
	assert.Equal(t, int64(24), hours)
	assert.True(t, ok)
}

func TestParseUserMfaRememberHours_UnknownKey(t *testing.T) {
	hours, ok := parseUserMfaRememberHours(map[string]any{"otherKey": 24})
	assert.Equal(t, int64(0), hours)
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// Tests for buildUserMfaSetupConfig additional cases
// ---------------------------------------------------------------------------

func TestBuildUserMfaSetupConfig_Radius(t *testing.T) {
	config, err := buildUserMfaSetupConfig("radius", "provider1", "radiususer", "")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "radius", config.MfaType)
	assert.Equal(t, "radiususer", config.Secret)
	assert.Equal(t, "provider1", config.URL)
}

func TestBuildUserMfaSetupConfig_Radius_MissingUsername(t *testing.T) {
	config, err := buildUserMfaSetupConfig("radius", "provider1", "", "")
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestBuildUserMfaSetupConfig_Radius_MissingProvider(t *testing.T) {
	config, err := buildUserMfaSetupConfig("radius", "", "radiususer", "")
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestBuildUserMfaSetupConfig_Push(t *testing.T) {
	config, err := buildUserMfaSetupConfig("push", "provider1", "receiver1", "")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "push", config.MfaType)
	assert.Equal(t, "receiver1", config.Secret)
	assert.Equal(t, "provider1", config.URL)
}

func TestBuildUserMfaSetupConfig_Push_MissingReceiver(t *testing.T) {
	config, err := buildUserMfaSetupConfig("push", "provider1", "", "")
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestBuildUserMfaSetupConfig_Push_MissingProvider(t *testing.T) {
	config, err := buildUserMfaSetupConfig("push", "", "receiver1", "")
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestBuildUserMfaSetupConfig_Totp_MissingSecret(t *testing.T) {
	config, err := buildUserMfaSetupConfig("app", "", "", "")
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestBuildUserMfaSetupConfig_Email_MissingDest(t *testing.T) {
	config, err := buildUserMfaSetupConfig("email", "", "", "")
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestBuildUserMfaSetupConfig_Sms_MissingDest(t *testing.T) {
	config, err := buildUserMfaSetupConfig("sms", "", "", "1")
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestBuildUserMfaSetupConfig_Sms_MissingCountryCode(t *testing.T) {
	config, err := buildUserMfaSetupConfig("sms", "", "+1234567890", "")
	assert.Error(t, err)
	assert.Nil(t, config)
}

// ---------------------------------------------------------------------------
// Tests for normalizeUserMfaSetupType additional cases
// ---------------------------------------------------------------------------

func TestNormalizeUserMfaSetupType_App(t *testing.T) {
	methodType, secretType, err := normalizeUserMfaSetupType("app")
	assert.NoError(t, err)
	assert.Equal(t, "app", methodType)
	assert.Equal(t, "totp", secretType)
}

func TestNormalizeUserMfaSetupType_Radius(t *testing.T) {
	methodType, secretType, err := normalizeUserMfaSetupType("radius")
	assert.NoError(t, err)
	assert.Equal(t, "radius", methodType)
	assert.Equal(t, "radius", secretType)
}

func TestNormalizeUserMfaSetupType_Push(t *testing.T) {
	methodType, secretType, err := normalizeUserMfaSetupType("push")
	assert.NoError(t, err)
	assert.Equal(t, "push", methodType)
	assert.Equal(t, "push", secretType)
}

func TestNormalizeUserMfaSetupType_Whitespace(t *testing.T) {
	methodType, secretType, err := normalizeUserMfaSetupType("  totp  ")
	assert.NoError(t, err)
	assert.Equal(t, "app", methodType)
	assert.Equal(t, "totp", secretType)
}

// ---------------------------------------------------------------------------
// Tests for userMfaMethodDisplayName additional cases
// ---------------------------------------------------------------------------

func TestUserMfaMethodDisplayName_Radius(t *testing.T) {
	assert.Equal(t, "RADIUS", userMfaMethodDisplayName("radius"))
}

func TestUserMfaMethodDisplayName_Push(t *testing.T) {
	assert.Equal(t, "Push", userMfaMethodDisplayName("push"))
}

func TestUserMfaMethodDisplayName_Unknown(t *testing.T) {
	assert.Equal(t, "CUSTOM", userMfaMethodDisplayName("custom"))
}

// ---------------------------------------------------------------------------
// Tests for normalizeExistingUserMfaMethodType
// ---------------------------------------------------------------------------

func TestNormalizeExistingUserMfaMethodType_App(t *testing.T) {
	assert.Equal(t, "totp", normalizeExistingUserMfaMethodType("app"))
}

func TestNormalizeExistingUserMfaMethodType_Empty(t *testing.T) {
	assert.Equal(t, "", normalizeExistingUserMfaMethodType(""))
}

func TestNormalizeExistingUserMfaMethodType_Whitespace(t *testing.T) {
	assert.Equal(t, "sms", normalizeExistingUserMfaMethodType("  sms  "))
}

// ---------------------------------------------------------------------------
// Tests for Service.InitiateUserMfaSetup with UOW mock
// ---------------------------------------------------------------------------

func TestService_InitiateUserMfaSetup_InvalidMfaType(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	_, err := svc.InitiateUserMfaSetup(ctx(), InitiateUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "invalid",
	})
	assert.Error(t, err)
}

func TestService_InitiateUserMfaSetup_MissingUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	_, err := svc.InitiateUserMfaSetup(ctx(), InitiateUserMfaSetupCommand{
		OrganizationID: "org-1",
		MfaType:        "totp",
	})
	assert.Error(t, err)
}

func TestService_InitiateUserMfaSetup_UserNotFound(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return nil, apperrors.NewNotFound("user not found")
				},
			},
		},
	}
	_, err := svc.InitiateUserMfaSetup(ctx(), InitiateUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "nonexistent",
		MfaType:        "totp",
	})
	assert.Error(t, err)
}

func TestService_InitiateUserMfaSetup_Success(t *testing.T) {
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        userdomain.UserProfile{CountryCode: "US"},
	}
	org := &orgdomain.Organization{
		ID:          "org-1",
		Slug:        "testorg",
		DisplayName: "Test Org",
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "recovery-code-1"},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return user, nil
				},
			},
			organizations: &svcMockOrgRepo{
				getByIDFn: func(_ context.Context, id string) (*orgdomain.Organization, error) {
					return org, nil
				},
			},
		},
	}
	result, err := svc.InitiateUserMfaSetup(ctx(), InitiateUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Secret)
	assert.NotEmpty(t, result.URL)
	assert.Contains(t, result.RecoveryCodes, "recovery-code-1")
	assert.Equal(t, "US", result.CountryCode)
}

func TestService_InitiateUserMfaSetup_OrgWithEmptyDisplayName(t *testing.T) {
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        userdomain.UserProfile{CountryCode: "US"},
	}
	org := &orgdomain.Organization{
		ID:          "org-1",
		Slug:        "myslug",
		DisplayName: "   ",
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "recovery-code-1"},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return user, nil
				},
			},
			organizations: &svcMockOrgRepo{
				getByIDFn: func(_ context.Context, id string) (*orgdomain.Organization, error) {
					return org, nil
				},
			},
		},
	}
	result, err := svc.InitiateUserMfaSetup(ctx(), InitiateUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestService_InitiateUserMfaSetup_OrgNotFound(t *testing.T) {
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        userdomain.UserProfile{},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "recovery-code-1"},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return user, nil
				},
			},
			organizations: &svcMockOrgRepo{
				getByIDFn: func(_ context.Context, id string) (*orgdomain.Organization, error) {
					return nil, apperrors.NewNotFound("organization not found")
				},
			},
		},
	}
	_, err := svc.InitiateUserMfaSetup(ctx(), InitiateUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
	})
	assert.Error(t, err)
}

func TestService_InitiateUserMfaSetup_NilOrg(t *testing.T) {
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        userdomain.UserProfile{CountryCode: "US"},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "recovery-code-1"},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return user, nil
				},
			},
			organizations: &svcMockOrgRepo{
				getByIDFn: func(_ context.Context, id string) (*orgdomain.Organization, error) {
					return nil, nil // org found but nil
				},
			},
		},
	}
	result, err := svc.InitiateUserMfaSetup(ctx(), InitiateUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// Tests for Service.VerifyUserMfaSetup
// ---------------------------------------------------------------------------

func TestService_VerifyUserMfaSetup_InvalidMfaType(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.VerifyUserMfaSetup(ctx(), VerifyUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "invalid",
	})
	assert.Error(t, err)
}

func TestService_VerifyUserMfaSetup_MissingUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.VerifyUserMfaSetup(ctx(), VerifyUserMfaSetupCommand{
		OrganizationID: "org-1",
		MfaType:        "totp",
	})
	assert.Error(t, err)
}

func TestService_VerifyUserMfaSetup_UserNotFound(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return nil, apperrors.NewNotFound("user not found")
				},
			},
		},
	}
	err := svc.VerifyUserMfaSetup(ctx(), VerifyUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "nonexistent",
		MfaType:        "totp",
		Passcode:       "123456",
		Secret:         "JBSWY3DPEHPK3PXP",
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Tests for Service.EnableUserMfaSetup
// ---------------------------------------------------------------------------

func TestService_EnableUserMfaSetup_InvalidMfaType(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "invalid",
	})
	assert.Error(t, err)
}

func TestService_EnableUserMfaSetup_MissingUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		MfaType:        "totp",
	})
	assert.Error(t, err)
}

func TestService_EnableUserMfaSetup_UserNotFound(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return nil, apperrors.NewNotFound("user not found")
				},
			},
		},
	}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "nonexistent",
		MfaType:        "totp",
		Secret:         "secret123",
	})
	assert.Error(t, err)
}

func TestService_EnableUserMfaSetup_Success_Totp(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        userdomain.UserProfile{},
		MfaMethods:     []userdomain.UserMfaMethod{},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "method-id-1"},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return user, nil
				},
				updateFn: func(_ context.Context, u *userdomain.User) error {
					return nil
				},
				replaceMfaMethodsFn: func(_ context.Context, userID string, methods []userdomain.UserMfaMethod) error {
					return nil
				},
			},
			organizations: &svcMockOrgRepo{
				getByIDFn: func(_ context.Context, id string) (*orgdomain.Organization, error) {
					return &orgdomain.Organization{ID: id, Slug: "built-in", DisplayName: "Built In"}, nil
				},
			},
		},
	}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
		Secret:         secret,
		Passcode:       validTotpCode(t, secret, timeNow()),
		RecoveryCodes:  []string{"recovery1"},
	})
	assert.NoError(t, err)
	assert.True(t, user.MfaMethods[0].Verified)
	assert.True(t, user.MfaMethods[0].Enabled)
	assert.NotNil(t, user.MfaMethods[0].LastUsedAt)

	// Server-owned collections mirror: the TOTP ceremony derives the MfaItem /
	// MfaAccount entries from the real factor state.
	if assert.Len(t, user.MfaItems, 1) {
		assert.Equal(t, "totp", user.MfaItems[0].MfaType)
		assert.Equal(t, secret, user.MfaItems[0].Secret)
	}
	if assert.Len(t, user.MfaAccounts, 1) {
		assert.Equal(t, "built-in/alice", user.MfaAccounts[0].AccountName)
		assert.Equal(t, "Built In", user.MfaAccounts[0].Issuer)
		assert.Equal(t, secret, user.MfaAccounts[0].SecretKey)
		assert.Equal(t, "totp", user.MfaAccounts[0].Origin)
	}
}

func TestService_EnableUserMfaSetup_Success_Email(t *testing.T) {
	original := emailSetupVerify
	emailSetupVerify = func(secret, passcode string) error { return nil }
	t.Cleanup(func() { emailSetupVerify = original })

	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        userdomain.UserProfile{Email: "alice@example.com", EmailVerified: true},
		MfaMethods:     []userdomain.UserMfaMethod{},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "method-id-1"},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return user, nil
				},
				updateFn: func(_ context.Context, u *userdomain.User) error {
					return nil
				},
				updateProfileFn: func(_ context.Context, p *userdomain.UserProfile) error {
					return nil
				},
				replaceMfaMethodsFn: func(_ context.Context, userID string, methods []userdomain.UserMfaMethod) error {
					return nil
				},
			},
		},
	}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "email",
		Dest:           "alice@example.com",
		Passcode:       "email-code",
		RecoveryCodes:  []string{"recovery1"},
	})
	assert.NoError(t, err)
}

func TestService_EnableUserMfaSetup_Success_SMS(t *testing.T) {
	original := smsSetupVerify
	smsSetupVerify = func(secret, countryCode, passcode string) error { return nil }
	t.Cleanup(func() { smsSetupVerify = original })

	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        userdomain.UserProfile{Phone: "+1234567890", CountryCode: "1"},
		MfaMethods:     []userdomain.UserMfaMethod{},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "method-id-1"},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return user, nil
				},
				updateFn: func(_ context.Context, u *userdomain.User) error {
					return nil
				},
				replaceMfaMethodsFn: func(_ context.Context, userID string, methods []userdomain.UserMfaMethod) error {
					return nil
				},
			},
		},
	}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "sms",
		Dest:           "+1234567890",
		CountryCode:    "1",
		Passcode:       "sms-code",
		RecoveryCodes:  []string{"recovery1"},
	})
	assert.NoError(t, err)
}

func TestService_EnableUserMfaSetup_UpdateError(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        userdomain.UserProfile{},
		MfaMethods:     []userdomain.UserMfaMethod{},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "method-id-1"},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return user, nil
				},
				updateFn: func(_ context.Context, u *userdomain.User) error {
					return apperrors.NewInternal("update failed")
				},
			},
		},
	}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
		Secret:         secret,
		Passcode:       validTotpCode(t, secret, timeNow()),
		RecoveryCodes:  []string{"recovery1"},
	})
	assert.Error(t, err)
}

func TestService_EnableUserMfaSetup_WrongTotpPasscodeKeepsState(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		MfaMethods: []userdomain.UserMfaMethod{
			{ID: "method-1", UserID: "user-1", MethodType: "totp", Enabled: true, Verified: true},
		},
	}
	writeCalled := false
	validCode := validTotpCode(t, secret, timeNow())
	wrongCode := "000000"
	if wrongCode == validCode {
		wrongCode = "111111"
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "method-id-1"},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return user, nil
				},
				updateFn: func(_ context.Context, u *userdomain.User) error {
					writeCalled = true
					return nil
				},
				replaceMfaMethodsFn: func(_ context.Context, userID string, methods []userdomain.UserMfaMethod) error {
					writeCalled = true
					return nil
				},
			},
		},
	}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
		Secret:         secret,
		Passcode:       wrongCode,
		RecoveryCodes:  []string{"recovery1"},
	})
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	assert.False(t, writeCalled, "a rejected passcode must not mutate the user")
	assert.True(t, user.MfaMethods[0].Enabled)
	assert.True(t, user.MfaMethods[0].Verified)
}

func TestService_EnableUserMfaSetup_ReplayedTotpPasscodeIsRejected(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	code := validTotpCode(t, secret, timeNow())
	step := timeNow().Unix() / int64(mfatotp.Period/time.Second)
	usedAt := mfatotp.StepStart(step)
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		MfaMethods: []userdomain.UserMfaMethod{
			{ID: "method-1", UserID: "user-1", MethodType: "totp", Enabled: true, Verified: true, LastUsedAt: &usedAt},
		},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "method-id-1"},
		UOW:   svcTestUnitOfWork{users: &svcMockUserRepo{getByIDFn: func(context.Context, string) (*userdomain.User, error) { return user, nil }}},
	}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
		Secret:         secret,
		Passcode:       code,
		RecoveryCodes:  []string{"recovery1"},
	})
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected replayed code to be unauthorized, got %v", err)
	}
}

func TestService_EnableUserMfaSetup_ExpiredTotpPasscodeIsRejected(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	user := &userdomain.User{ID: "user-1", OrganizationID: "org-1"}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "method-id-1"},
		UOW:   svcTestUnitOfWork{users: &svcMockUserRepo{getByIDFn: func(context.Context, string) (*userdomain.User, error) { return user, nil }}},
	}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
		Secret:         secret,
		Passcode:       validTotpCode(t, secret, timeNow().Add(-5*time.Minute)),
		RecoveryCodes:  []string{"recovery1"},
	})
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected expired code to be unauthorized, got %v", err)
	}
}

func TestService_EnableUserMfaSetup_PushHasNoVerificationIsRejected(t *testing.T) {
	user := &userdomain.User{ID: "user-1", OrganizationID: "org-1"}
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "method-id-1"},
		UOW:   svcTestUnitOfWork{users: &svcMockUserRepo{getByIDFn: func(context.Context, string) (*userdomain.User, error) { return user, nil }}},
	}
	_, err := svc.EnableUserMfaSetup(ctx(), EnableUserMfaSetupCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "push",
		Dest:           "receiver",
		Secret:         "provider",
		Passcode:       "challenge",
		RecoveryCodes:  []string{"recovery1"},
	})
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected unimplemented push verification, got %v", err)
	}
}

func assertRecoveryHashesMatch(t *testing.T, hashes []string, codes ...string) {
	t.Helper()
	if len(hashes) != len(codes) {
		t.Fatalf("hash count = %d, want %d", len(hashes), len(codes))
	}
	for _, code := range codes {
		matched := false
		for _, hash := range hashes {
			ok, err := platformcrypto.VerifyClientSecret(code, hash)
			if err != nil {
				t.Fatalf("verify recovery hash: %v", err)
			}
			if ok {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("no stored hash matches recovery code %q", code)
		}
	}
}
