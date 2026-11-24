package users

import (
	"encoding/json"
	"testing"
	"time"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestSafeGet_InBounds(t *testing.T) {
	assert.Equal(t, "a", safeGet([]string{"a", "b", "c"}, 0))
	assert.Equal(t, "b", safeGet([]string{"a", "b", "c"}, 1))
}

func TestSafeGet_OutOfBounds(t *testing.T) {
	assert.Equal(t, "", safeGet([]string{"a", "b"}, 5))
	assert.Equal(t, "", safeGet([]string{}, 0))
}

func TestResolveCSVField_WithMapping(t *testing.T) {
	rowMap := map[string]string{"name": "alice", "email": "alice@example.com"}
	mapping := map[string]string{"name": "displayName", "email": "mail"}

	assert.Equal(t, "alice", resolveCSVField(rowMap, mapping, "displayName"))
	assert.Equal(t, "alice@example.com", resolveCSVField(rowMap, mapping, "mail"))
	assert.Equal(t, "", resolveCSVField(rowMap, mapping, "unknown"))
}

func TestResolveCSVField_NoMapping(t *testing.T) {
	rowMap := map[string]string{"name": "alice"}
	mapping := map[string]string{}

	assert.Equal(t, "alice", resolveCSVField(rowMap, mapping, "name"))
}

func TestNormalizeSetupSummaryMethodType(t *testing.T) {
	assert.Equal(t, "app", normalizeSetupSummaryMethodType("totp")) // "totp" is normalized to "app"
	assert.Equal(t, "sms", normalizeSetupSummaryMethodType("sms"))
	assert.Equal(t, "email", normalizeSetupSummaryMethodType("email"))
}

func TestNormalizeExistingUserMfaMethodType(t *testing.T) {
	assert.Equal(t, "totp", normalizeExistingUserMfaMethodType("totp"))
	assert.Equal(t, "sms", normalizeExistingUserMfaMethodType("sms"))
}

func TestUserMfaMethodDisplayName(t *testing.T) {
	assert.Equal(t, "Authenticator App", userMfaMethodDisplayName("totp"))
	assert.Equal(t, "SMS", userMfaMethodDisplayName("sms"))
	assert.Equal(t, "Email", userMfaMethodDisplayName("email"))
}

func TestToListItem(t *testing.T) {
	now := time.Now()
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		IsForbidden:    false,
		Profile: userdomain.UserProfile{
			DisplayName: "Alice Smith",
			Email:       "alice@example.com",
			Phone:       "+1234567890",
			Properties:  map[string]any{"karma": 42, "ranking": "7"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	item := toListItem(user)
	assert.Equal(t, "user-1", item.ID)
	assert.Equal(t, "org-1", item.OrganizationID)
	assert.Equal(t, "alice", item.Username)
	assert.Equal(t, "Alice Smith", item.DisplayName)
	assert.Equal(t, "alice@example.com", item.Email)
	// 规范 9.5：手机号列表查询时掩码中间 4 位。
	assert.Equal(t, "+123****7890", item.Phone)
	assert.False(t, item.IsForbidden)
	assert.Equal(t, 42, item.Karma)
	assert.Equal(t, 7, item.Ranking)
}

func TestToDetailIncludesExtendedProfileProperties(t *testing.T) {
	user := &userdomain.User{
		Profile: userdomain.UserProfile{
			Properties: map[string]any{
				"homepage":  "https://example.com",
				"education": "University",
				"language":  "en",
				"title":     "Engineer",
				"karma":     42,
				"ranking":   "7",
			},
		},
	}

	detail := toDetail(user, nil)
	assert.Equal(t, 42, detail.Karma)
	assert.Equal(t, 7, detail.Ranking)
	assert.Equal(t, "https://example.com", detail.Profile["homepage"])
	assert.Equal(t, "University", detail.Profile["education"])
	assert.Equal(t, "en", detail.Profile["language"])
	assert.Equal(t, "Engineer", detail.Profile["title"])
}

func TestToDetailIncludesCertificateProfileColumns(t *testing.T) {
	user := &userdomain.User{
		Profile: userdomain.UserProfile{
			RealName:              "Zhang San",
			IDCard:                "110101199001011234",
			PassportNo:            "E12345678",
			DriverLicenseNo:       "D1234567",
			IDCardExpiry:          "2030-12-31",
			IssueCountry:          "CN",
			IssuingAuthority:      "MPS Beijing",
			PassportPhotoURL:      "https://cdn.example.com/passport.png",
			DriverLicenseFrontURL: "https://cdn.example.com/dl-front.png",
			DriverLicenseBackURL:  "https://cdn.example.com/dl-back.png",
		},
	}

	detail := toDetail(user, nil)
	for key, want := range map[string]string{
		"passportNoMasked":      "******",
		"driverLicenseNoMasked": "******",
		"idCardMasked":          "******",
		"idCardExpiry":          "2030-12-31",
		"issueCountry":          "CN",
		"issuingAuthority":      "MPS Beijing",
		"passportPhotoUrl":      "https://cdn.example.com/passport.png",
		"driverLicenseFrontUrl": "https://cdn.example.com/dl-front.png",
		"driverLicenseBackUrl":  "https://cdn.example.com/dl-back.png",
	} {
		assert.Equal(t, want, detail.Profile[key], key)
	}
	// 规范 9.5：证件号原文永不进入响应。
	for _, key := range []string{"idCard", "passportNo", "driverLicenseNo"} {
		assert.NotContains(t, detail.Profile, key)
	}
	for _, key := range []string{"idCardSet", "passportNoSet", "driverLicenseNoSet"} {
		assert.Equal(t, true, detail.Profile[key], key)
	}
}

// 存量 properties JSONB 可能携带证件号原文键（保留键禁令之前的历史数据）：
// 响应侧必须剔除，否则 Set/Masked 脱敏被 properties 通道旁路。
func TestToDetailStripsLegacyDocumentNumbersFromProperties(t *testing.T) {
	user := &userdomain.User{
		Profile: userdomain.UserProfile{
			Properties: map[string]any{
				"idCard":          "110101199001011234",
				"passportNo":      "E12345678",
				"driverLicenseNo": "D1234567",
				"homepage":        "https://example.com",
			},
		},
	}

	detail := toDetail(user, nil)
	props, ok := detail.Profile["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties missing from profile: %+v", detail.Profile)
	}
	for _, key := range []string{"idCard", "passportNo", "driverLicenseNo"} {
		assert.NotContains(t, props, key)
	}
	assert.Equal(t, "https://example.com", props["homepage"])
}

func TestToDetailMasksStoredCredentials(t *testing.T) {
	user := &userdomain.User{
		ManagedAccounts: []userdomain.ManagedAccount{
			{Application: "app", Username: "alice", Password: "managed-password"},
		},
		MfaItems: []userdomain.MfaItem{
			{MfaType: "totp", Secret: "mfa-item-secret"},
		},
		MfaAccounts: []userdomain.MfaAccount{
			{AccountName: "alice", Issuer: "issuer", SecretKey: "totp-secret-key"},
		},
		Credentials: userdomain.UserCredentials{AccessKey: "user-access-key"},
	}

	detail := toDetail(user, nil)
	raw, err := json.Marshal(detail)
	assert.NoError(t, err)
	body := string(raw)

	for _, secret := range []string{"managed-password", "mfa-item-secret", "totp-secret-key", "user-access-key"} {
		assert.NotContains(t, body, secret)
	}
	for _, field := range []string{`"password":`, `"secretKey":`, `"accessKeyPreview"`} {
		assert.NotContains(t, body, field)
	}
	assert.Contains(t, body, `"passwordSet":true`)
	assert.Contains(t, body, `"passwordMasked":"******"`)
	assert.Contains(t, body, `"secretSet":true`)
	assert.Contains(t, body, `"secretMasked":"******"`)
	assert.Contains(t, body, `"secretKeySet":true`)
	assert.Contains(t, body, `"secretKeyMasked":"******"`)
	assert.Contains(t, body, `"accessKeySet":true`)
	assert.Contains(t, body, `"accessKeyMasked":"******"`)
	assert.NotContains(t, body, `"accessKeyPreview"`)
}

func TestToListItem_ForbiddenUser(t *testing.T) {
	user := &userdomain.User{
		Username:    "bob",
		IsForbidden: true,
	}
	item := toListItem(user)
	assert.True(t, item.IsForbidden)
}

func TestNormalizeUserMfaSetupType_Totp(t *testing.T) {
	methodType, secretType, err := normalizeUserMfaSetupType("totp")
	assert.NoError(t, err)
	assert.Equal(t, "app", methodType) // "totp" is normalized to "app"
	assert.Equal(t, "totp", secretType)
}

func TestNormalizeUserMfaSetupType_Sms(t *testing.T) {
	methodType, secretType, err := normalizeUserMfaSetupType("sms")
	assert.NoError(t, err)
	assert.Equal(t, "sms", methodType)
	assert.Equal(t, "sms", secretType)
}

func TestNormalizeUserMfaSetupType_Email(t *testing.T) {
	methodType, secretType, err := normalizeUserMfaSetupType("email")
	assert.NoError(t, err)
	assert.Equal(t, "email", methodType)
	assert.Equal(t, "email", secretType)
}

func TestNormalizeUserMfaSetupType_Invalid(t *testing.T) {
	_, _, err := normalizeUserMfaSetupType("invalid-type")
	appErr := assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
	if appErr.Meta["field"] != "mfaType" || appErr.Meta["allowed"] == nil {
		t.Fatalf("meta = %v, want field=mfaType + allowed", appErr.Meta)
	}
}

func TestBuildUserMfaSetupConfig_Totp(t *testing.T) {
	config, err := buildUserMfaSetupConfig("totp", "secret123", "", "")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "totp", config.MfaType)

}

func TestBuildUserMfaSetupConfig_Sms(t *testing.T) {
	config, err := buildUserMfaSetupConfig("sms", "secret123", "+1234567890", "1")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "sms", config.MfaType)
}

func TestBuildUserMfaSetupConfig_Email(t *testing.T) {
	config, err := buildUserMfaSetupConfig("email", "secret123", "user@example.com", "")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "email", config.MfaType)
}
