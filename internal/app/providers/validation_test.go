package providers

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	domainnotification "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wantAppProviderError(t *testing.T, err error, status int) *apperrors.Error {
	t.Helper()
	require.Error(t, err)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok, "guard must return *apperrors.Error, got %T", err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, status, appErr.HTTPStatus())
	return appErr
}

func TestValidateIdentityKind(t *testing.T) {
	require.NoError(t, validateIdentityKind(domain.Kind("github")))

	appErr := wantAppProviderError(t, validateIdentityKind(domain.Kind("nope")), 422)
	assert.Equal(t, "kind", appErr.Meta["field"])
	assert.Equal(t, "nope", appErr.Meta["value"])
	assert.Len(t, appErr.Meta["allowed"], len(identityKinds))
}

func TestValidatePaymentKind(t *testing.T) {
	require.NoError(t, validatePaymentKind(domain.Kind("alipay")))
	require.NoError(t, validatePaymentKind(domain.Kind("wechat_pay")))

	appErr := wantAppProviderError(t, validatePaymentKind(domain.Kind("wxpay")), 422)
	assert.Equal(t, "kind", appErr.Meta["field"])
	assert.Equal(t, "wxpay", appErr.Meta["value"])
	assert.Equal(t, paymentKinds, appErr.Meta["allowed"])
}

func TestValidateNotificationKind(t *testing.T) {
	require.NoError(t, validateNotificationKind(domain.Kind("email")))

	appErr := wantAppProviderError(t, validateNotificationKind(domain.Kind("wechat")), 422)
	assert.Equal(t, "kind", appErr.Meta["field"])
	assert.Equal(t, "wechat", appErr.Meta["value"])
	assert.Equal(t, "notificationProviderType", appErr.Meta["set"])
	assert.Equal(t, domainnotification.ProviderTypes.SortedValues(), appErr.Meta["allowed"])
}

func TestValidatePaymentEnvironment(t *testing.T) {
	require.NoError(t, validatePaymentEnvironment("production", domain.VariantProduction))

	appErr := wantAppProviderError(t, validatePaymentEnvironment("sandbox", domain.VariantProduction), 422)
	assert.Equal(t, map[string]any{
		"field":    "environment",
		"value":    "sandbox",
		"expected": "production",
	}, appErr.Meta)
}

func TestValidateActivationReadiness(t *testing.T) {
	passedAt := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, validateActivationReadiness(&passedAt))

	appErr := wantAppProviderError(t, validateActivationReadiness(nil), 422)
	assert.Equal(t, "testPassedAt", appErr.Meta["field"])
}

func TestValidateConfigSealed(t *testing.T) {
	appErr := wantAppProviderError(t, validateConfigSealed(), 422)
	assert.Equal(t, "configSealed", appErr.Meta["field"])
	assert.Equal(t, "envelope", appErr.Meta["format"])
}

func TestValidateChangeStatus(t *testing.T) {
	require.NoError(t, validateChangeStatus(domain.StatusActive))
	require.NoError(t, validateChangeStatus(domain.StatusDisabled))

	appErr := wantAppProviderError(t, validateChangeStatus(domain.StatusDraft), 422)
	assert.Equal(t, map[string]any{
		"field":   "status",
		"value":   "draft",
		"allowed": []string{"active", "disabled"},
	}, appErr.Meta)
}

func TestValidateSecretType(t *testing.T) {
	for _, secretType := range []string{
		secretTypeClientSecret, secretTypeClientSecret2, secretTypePassword, secretTypeAppSecret,
	} {
		require.NoError(t, validateSecretType(domain.FamilyIdentity, secretType))
	}
	require.NoError(t, validateSecretType(domain.FamilyAI, secretTypeAPIKey))
	require.NoError(t, validateSecretType(domain.FamilyNotification, secretTypeAccessToken))
	require.NoError(t, validateSecretType(domain.FamilyPayment, secretTypeConfig))

	appErr := wantAppProviderError(t, validateSecretType(domain.FamilyIdentity, secretTypeConfig), 422)
	assert.Equal(t, "secretType", appErr.Meta["field"])
	assert.Equal(t, "config", appErr.Meta["value"])
	assert.Equal(t, []string{
		secretTypeClientSecret, secretTypeClientSecret2, secretTypePassword, secretTypeAppSecret,
	}, appErr.Meta["allowed"])
}

func TestValidateOptionalURL(t *testing.T) {
	require.NoError(t, validateOptionalURL("", "providerUrl"))
	require.NoError(t, validateOptionalURL("https://idp.example.com", "providerUrl"))

	appErr := wantAppProviderError(t, validateOptionalURL("not a url", "providerUrl"), 422)
	assert.Equal(t, map[string]any{
		"field":  "providerUrl",
		"value":  "not a url",
		"format": "uri",
	}, appErr.Meta)

	for _, raw := range []string{
		"https://user:pw@example.com/cb",
		"https://example.com/cb#frag",
	} {
		appErr = wantAppProviderError(t, validateOptionalURL(raw, "providerUrl"), 422)
		assert.Equal(t, "providerUrl", appErr.Meta["field"])
		assert.Equal(t, raw, appErr.Meta["value"])
		assert.Equal(t, "uri", appErr.Meta["format"])
	}
}

func TestValidateOptionalCallbackURL(t *testing.T) {
	require.NoError(t, validateOptionalCallbackURL(""))
	require.NoError(t, validateOptionalCallbackURL("   "))
	require.NoError(t, validateOptionalCallbackURL("https://iam.example.com/oauth/callback"))
	require.NoError(t, validateOptionalCallbackURL("http://localhost:8001/callback"))

	appErr := wantAppProviderError(t, validateOptionalCallbackURL("iam.example.com/callback"), 422)
	assert.Equal(t, "callbackUrl", appErr.Meta["field"])
	assert.Equal(t, "iam.example.com/callback", appErr.Meta["value"])
	assert.Equal(t, "uri", appErr.Meta["format"])

	appErr = wantAppProviderError(t, validateOptionalCallbackURL("javascript:alert(1)"), 422)
	assert.Equal(t, "callbackUrl", appErr.Meta["field"])

	for _, raw := range []string{
		"https://user:pw@example.com/cb",
		"https://example.com/cb#frag",
	} {
		appErr = wantAppProviderError(t, validateOptionalCallbackURL(raw), 422)
		assert.Equal(t, "callbackUrl", appErr.Meta["field"])
		assert.Equal(t, raw, appErr.Meta["value"])
		assert.Equal(t, "uri", appErr.Meta["format"])
	}

	tooLong := "https://iam.example.com/" + strings.Repeat("a", 255)
	appErr = wantAppProviderError(t, validateOptionalCallbackURL(tooLong), 422)
	assert.Equal(t, "callbackUrl", appErr.Meta["field"])
	assert.Equal(t, maxCallbackURLLength, appErr.Meta["max"])
}

func TestNormalizeProviderScopes(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		scopes, err := normalizeProviderScopes(nil)
		require.NoError(t, err)
		assert.Empty(t, scopes)

		scopes, err = normalizeProviderScopes([]string{"", "   "})
		require.NoError(t, err)
		assert.Empty(t, scopes)
	})

	t.Run("splits_trims_and_dedupes", func(t *testing.T) {
		scopes, err := normalizeProviderScopes([]string{" openid  profile ", "email,openid", "email"})
		require.NoError(t, err)
		assert.Equal(t, []string{"openid", "profile", "email"}, scopes)
	})

	t.Run("rejects_invalid_token", func(t *testing.T) {
		_, err := normalizeProviderScopes([]string{"openid", `bad"scope`})
		appErr := wantAppProviderError(t, err, 422)
		assert.Equal(t, "scopes", appErr.Meta["field"])
		assert.Equal(t, 1, appErr.Meta["index"])
		assert.Equal(t, `bad"scope`, appErr.Meta["value"])
		assert.Equal(t, "scope-token", appErr.Meta["format"])

		_, err = normalizeProviderScopes([]string{"bad\\scope"})
		wantAppProviderError(t, err, 422)
	})

	t.Run("rejects_overlong_token", func(t *testing.T) {
		_, err := normalizeProviderScopes([]string{strings.Repeat("a", maxProviderScopeLen+1)})
		appErr := wantAppProviderError(t, err, 422)
		assert.Equal(t, "scopes", appErr.Meta["field"])
		assert.Equal(t, maxProviderScopeLen, appErr.Meta["max"])
	})

	t.Run("rejects_too_many_tokens", func(t *testing.T) {
		values := make([]string, 0, maxProviderScopes+1)
		for i := 0; i <= maxProviderScopes; i++ {
			values = append(values, "scope"+strconv.Itoa(i))
		}
		_, err := normalizeProviderScopes(values)
		appErr := wantAppProviderError(t, err, 422)
		assert.Equal(t, "scopes", appErr.Meta["field"])
		assert.Equal(t, maxProviderScopes, appErr.Meta["max"])
	})
}

func TestValidateCreateProviderCommandCallbackURL(t *testing.T) {
	valid := CreateProviderCommand{
		OrganizationID: "org_1",
		Name:           "github",
		DisplayName:    "GitHub",
		Category:       "OAuth",
		ProviderType:   "github",
		ProviderURL:    "https://github.com",
		CallbackUrl:    "https://iam.example.com/oauth/callback",
		Scopes:         []string{"openid", "profile"},
	}
	require.NoError(t, validateCreateProviderCommand(valid))

	invalid := valid
	invalid.CallbackUrl = "javascript:alert(1)"
	appErr := wantAppProviderError(t, validateCreateProviderCommand(invalid), 422)
	assert.Equal(t, "callbackUrl", appErr.Meta["field"])

	invalid = valid
	invalid.Scopes = []string{"openid profile", `bad"scope`}
	appErr = wantAppProviderError(t, validateCreateProviderCommand(invalid), 422)
	assert.Equal(t, "scopes", appErr.Meta["field"])
}

func TestValidatePatchProviderCommandStaysShapeOnly(t *testing.T) {
	empty := ""
	require.NoError(t, validatePatchProviderCommand(PatchProviderCommand{CallbackUrl: &empty}))
	// 取值校验已移到加载持久记录之后（validateProviderPatchWrite），这里对脏值
	// 不再提前拒绝，否则任何无关字段的 PATCH 都会被存量数据阻断。
	bad := "iam.example.com/callback"
	require.NoError(t, validatePatchProviderCommand(PatchProviderCommand{CallbackUrl: &bad}))
	require.NoError(t, validatePatchProviderCommand(PatchProviderCommand{Scopes: []string{`bad"scope`}}))
}

// seedDirtyIdentityProvider 写入一条带历史脏值（非法 providerUrl/callbackUrl/
// scopes）的身份提供商，用于验证写侧校验只作用于本次真正变化的字段。
func seedDirtyIdentityProvider(t *testing.T, repo *mockProviderRepo, now time.Time) {
	t.Helper()
	seedIdentityProvider(t, repo, now, "github", "github", domain.StatusDisabled)
	repo.providers[0].ProviderURL = "https://user:pw@idp.example.com"
	repo.providers[0].CallbackUrl = "https://user:pw@example.com/cb#frag"
	repo.providers[0].Scopes = []string{`bad"scope`}
}

func TestUnifiedPatchIgnoresDirtyStoredValuesOnUnrelatedEdit(t *testing.T) {
	now := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)
	svc, repo, _, _ := newUnifiedTestService(now)
	seedDirtyIdentityProvider(t, repo, now)

	name := "renamed-github"
	_, err := svc.Unified.Patch(context.Background(), PatchProviderCommand{
		ID: "provider-1", OrganizationID: "org-1", Name: &name,
	})
	require.NoError(t, err)
	assert.Equal(t, name, repo.providers[0].Name)
	assert.Equal(t, "https://user:pw@idp.example.com", repo.providers[0].ProviderURL)
	assert.Equal(t, "https://user:pw@example.com/cb#frag", repo.providers[0].CallbackUrl)
	assert.Equal(t, []string{`bad"scope`}, repo.providers[0].Scopes)
}

func TestUnifiedPatchRejectsChangedInvalidCallbackURL(t *testing.T) {
	now := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)
	seedDirtyIdentityProvider(t, repo, now)

	bad := "https://example.com/cb#frag"
	_, err := svc.Unified.Patch(context.Background(), PatchProviderCommand{
		ID: "provider-1", OrganizationID: "org-1", CallbackUrl: &bad,
	})
	appErr := wantAppProviderError(t, err, 422)
	assert.Equal(t, "callbackUrl", appErr.Meta["field"])
	assert.Equal(t, bad, appErr.Meta["value"])
	assert.Equal(t, "https://user:pw@example.com/cb#frag", repo.providers[0].CallbackUrl)
	assert.Empty(t, records.items)
	assert.Empty(t, outbox.items)
}

func TestUnifiedPatchKeepsUnchangedScopesVerbatim(t *testing.T) {
	now := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)
	svc, repo, _, _ := newUnifiedTestService(now)
	seedDirtyIdentityProvider(t, repo, now)
	// 旧值里带空格，直接 normalize 会拆成两个 token；原样回传必须保持不动。
	repo.providers[0].Scopes = []string{"openid profile"}

	_, err := svc.Unified.Patch(context.Background(), PatchProviderCommand{
		ID: "provider-1", OrganizationID: "org-1", Scopes: []string{"openid profile"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"openid profile"}, repo.providers[0].Scopes)
}

func TestUnifiedPatchNormalizesChangedScopes(t *testing.T) {
	now := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)
	svc, repo, _, _ := newUnifiedTestService(now)
	seedDirtyIdentityProvider(t, repo, now)
	repo.providers[0].Scopes = []string{"openid"}

	_, err := svc.Unified.Patch(context.Background(), PatchProviderCommand{
		ID: "provider-1", OrganizationID: "org-1", Scopes: []string{"profile, email", "profile"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"profile", "email"}, repo.providers[0].Scopes)
}

// createIdentityProviderRow 通过正常创建写出一条合法的身份提供商，再把持久行
// 改成指定 URL，模拟上线前遗留的脏数据：除 URL 外其余字段都满足启用校验。
func createIdentityProviderRow(t *testing.T, svc Service, repo *mockProviderRepo, providerURL, callback string) {
	t.Helper()
	_, err := svc.CreateProvider(context.Background(), createIdentityCommand("initial-secret"))
	require.NoError(t, err)
	require.Len(t, repo.providers, 1)
	repo.providers[0].ProviderURL = providerURL
	repo.providers[0].CallbackUrl = callback
}

func TestUnifiedActivateRejectsDirtyStoredURLs(t *testing.T) {
	now := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		providerURL string
		callback    string
		field       string
		value       string
	}{
		{
			name: "callback userinfo", callback: "https://user:pw@example.com/cb",
			field: "callbackUrl", value: "https://user:pw@example.com/cb",
		},
		{
			name: "callback fragment", callback: "https://example.com/cb#frag",
			field: "callbackUrl", value: "https://example.com/cb#frag",
		},
		{
			name: "provider userinfo", providerURL: "https://user:pw@idp.example.com", callback: "https://iam.example.com/cb",
			field: "providerUrl", value: "https://user:pw@idp.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, records, outbox := newUnifiedTestService(now)
			createIdentityProviderRow(t, svc, repo, tt.providerURL, tt.callback)
			recordCount, outboxCount := len(records.items), len(outbox.items)

			_, err := svc.EnableProvider(context.Background(), "provider-1", "org-1", "actor-1")
			appErr := wantAppProviderError(t, err, 422)
			assert.Equal(t, tt.field, appErr.Meta["field"])
			assert.Equal(t, tt.value, appErr.Meta["value"])
			assert.False(t, repo.providers[0].Enabled)

			metadata, err := getRegistryMetadata(context.Background(), repo, "provider-1")
			require.NoError(t, err)
			assert.Equal(t, string(domain.StatusDraft), metadata.Status)
			assert.Len(t, records.items, recordCount)
			assert.Len(t, outbox.items, outboxCount)
		})
	}
}

func TestUnifiedActivateAllowsCleanStoredURLs(t *testing.T) {
	now := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)
	createIdentityProviderRow(t, svc, repo, "https://github.com", "https://iam.example.com/oauth/callback")
	repo.providers[0].AuthUrl = "https://github.com/login/oauth/authorize"
	repo.providers[0].TokenUrl = "https://github.com/login/oauth/access_token"
	recordCount, outboxCount := len(records.items), len(outbox.items)

	_, err := svc.EnableProvider(context.Background(), "provider-1", "org-1", "actor-1")
	require.NoError(t, err)
	assert.True(t, repo.providers[0].Enabled)
	metadata, err := getRegistryMetadata(context.Background(), repo, "provider-1")
	require.NoError(t, err)
	assert.Equal(t, string(domain.StatusActive), metadata.Status)
	require.Len(t, records.items, recordCount+1)
	assert.Equal(t, "provider.identity.activated", records.items[recordCount].Action)
	require.Len(t, outbox.items, outboxCount+1)
}
