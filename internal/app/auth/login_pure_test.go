package auth

import (
	"testing"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestFirstNonEmpty(t *testing.T) {
	if v := firstNonEmpty("", "  ", "hello"); v != "hello" {
		t.Errorf("got %q, want hello", v)
	}
	if v := firstNonEmpty("", "world"); v != "world" {
		t.Errorf("got %q, want world", v)
	}
	if v := firstNonEmpty("", "  ", ""); v != "" {
		t.Errorf("got %q, want empty", v)
	}
	if v := firstNonEmpty("first", "second"); v != "first" {
		t.Errorf("got %q, want first", v)
	}
}

func TestStringConfigValue(t *testing.T) {
	if v := stringConfigValue(nil, "key"); v != "" {
		t.Errorf("expected empty for nil, got %q", v)
	}
	m := map[string]any{"key": "value"}
	if v := stringConfigValue(m, "key"); v != "value" {
		t.Errorf("got %q, want value", v)
	}
	m2 := map[string]any{"key": " spaced "}
	if v := stringConfigValue(m2, "key"); v != "spaced" {
		t.Errorf("got %q, want spaced", v)
	}
	if v := stringConfigValue(m, "missing"); v != "" {
		t.Errorf("expected empty for missing key, got %q", v)
	}
	m3 := map[string]any{"key": 42}
	if v := stringConfigValue(m3, "key"); v != "42" {
		t.Errorf("got %q, want 42", v)
	}
}

func TestIntConfigValue(t *testing.T) {
	if v := intConfigValue(nil, "key"); v != 0 {
		t.Errorf("expected 0 for nil, got %d", v)
	}
	m := map[string]any{"key": 42}
	if v := intConfigValue(m, "key"); v != 42 {
		t.Errorf("got %d, want 42", v)
	}
	m2 := map[string]any{"key": int64(99)}
	if v := intConfigValue(m2, "key"); v != 99 {
		t.Errorf("got %d, want 99", v)
	}
	m3 := map[string]any{"key": "100"}
	if v := intConfigValue(m3, "key"); v != 100 {
		t.Errorf("got %d, want 100", v)
	}
	if v := intConfigValue(m, "missing"); v != 0 {
		t.Errorf("expected 0 for missing key, got %d", v)
	}
}

func TestHostFromProviderURL(t *testing.T) {
	if v := hostFromProviderURL("https://example.com:8443"); v != "example.com" {
		t.Errorf("got %q, want example.com", v)
	}
	if v := hostFromProviderURL("example.com:8443"); v != "example.com" {
		t.Errorf("got %q, want example.com", v)
	}
	if v := hostFromProviderURL("example.com"); v != "example.com" {
		t.Errorf("got %q, want example.com", v)
	}
	if v := hostFromProviderURL(""); v != "" {
		t.Errorf("got %q, want empty", v)
	}
	if v := hostFromProviderURL("  "); v != "" {
		t.Errorf("got %q, want empty", v)
	}
}

func TestPortFromProviderURL(t *testing.T) {
	if v := portFromProviderURL("https://example.com:8443"); v != 8443 {
		t.Errorf("got %d, want 8443", v)
	}
	if v := portFromProviderURL("example.com:8443"); v != 8443 {
		t.Errorf("got %d, want 8443", v)
	}
	if v := portFromProviderURL("https://example.com"); v != 0 {
		t.Errorf("got %d, want 0 (no port)", v)
	}
	if v := portFromProviderURL(""); v != 0 {
		t.Errorf("got %d, want 0", v)
	}
}

func TestNormalizeSetupMfaType(t *testing.T) {
	if v := normalizeSetupMfaType("totp"); v != "app" {
		t.Errorf("got %q, want app", v)
	}
	if v := normalizeSetupMfaType("email"); v != "email" {
		t.Errorf("got %q, want email", v)
	}
	if v := normalizeSetupMfaType("sms"); v != "sms" {
		t.Errorf("got %q, want sms", v)
	}
	if v := normalizeSetupMfaType("push"); v != "push" {
		t.Errorf("got %q, want push", v)
	}
	if v := normalizeSetupMfaType("unknown"); v != "" {
		t.Errorf("got %q, want empty", v)
	}
	if v := normalizeSetupMfaType("  totp  "); v != "app" {
		t.Errorf("got %q, want app", v)
	}
}

func TestOrganizationRequiresMfa(t *testing.T) {
	if organizationRequiresMfa(nil) {
		t.Error("expected false for nil")
	}
	org := &orgdomain.Organization{}
	if organizationRequiresMfa(org) {
		t.Error("expected false for empty policy")
	}
	org2 := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			MfaPolicy: map[string]any{"required": true},
		},
	}
	if !organizationRequiresMfa(org2) {
		t.Error("expected true")
	}
	org3 := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			MfaPolicy: map[string]any{"required": "true"},
		},
	}
	if !organizationRequiresMfa(org3) {
		t.Error("expected true from string")
	}
	org4 := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			MfaPolicy: map[string]any{"required": false},
		},
	}
	if organizationRequiresMfa(org4) {
		t.Error("expected false")
	}
}

func TestRequiresMfaVerification(t *testing.T) {
	now := time.Now()
	user := &userdomain.User{}
	if !requiresMfaVerification(now, user) {
		t.Error("expected true when no deadline")
	}
	future := now.Add(24 * time.Hour)
	user.MfaRememberDeadline = &future
	if requiresMfaVerification(now, user) {
		t.Error("expected false when deadline in future")
	}
	past := now.Add(-1 * time.Hour)
	user.MfaRememberDeadline = &past
	if !requiresMfaVerification(now, user) {
		t.Error("expected true when deadline in past")
	}
}

func TestHasMfaVerificationAttempt(t *testing.T) {
	if hasMfaVerificationAttempt(LoginCommand{}) {
		t.Error("expected false for empty command")
	}
	if !hasMfaVerificationAttempt(LoginCommand{Passcode: "123456"}) {
		t.Error("expected true for passcode")
	}
	if !hasMfaVerificationAttempt(LoginCommand{RecoveryCode: "abc"}) {
		t.Error("expected true for recovery code")
	}
	if !hasMfaVerificationAttempt(LoginCommand{MfaType: "push"}) {
		t.Error("expected true for push type")
	}
}

func TestInvalidChallengeError(t *testing.T) {
	err := invalidChallengeError()
	if err == nil {
		t.Error("expected error")
	}
}

func TestIsUserAllowedToLogin(t *testing.T) {
	if !isUserAllowedToLogin("active", false) {
		t.Error("expected true for active status")
	}
	if !isUserAllowedToLogin("", false) {
		t.Error("expected true for empty status")
	}
	if isUserAllowedToLogin("active", true) {
		t.Error("expected false for forbidden")
	}
	if isUserAllowedToLogin("suspended", false) {
		t.Error("expected false for suspended")
	}
}

func TestCheckApplicationTagAccess_NoTags(t *testing.T) {
	app := &applicationdomain.Application{}
	user := &userdomain.User{}
	if err := checkApplicationTagAccess(app, user); err != nil {
		t.Errorf("expected nil for no tags: %v", err)
	}
}

func TestCheckApplicationTagAccess_WithTags(t *testing.T) {
	app := &applicationdomain.Application{Tags: []string{"vip", "admin"}}
	user := &userdomain.User{Tag: "vip"}
	if err := checkApplicationTagAccess(app, user); err != nil {
		t.Errorf("expected nil for matching tag: %v", err)
	}
	user2 := &userdomain.User{Tag: "basic"}
	if err := checkApplicationTagAccess(app, user2); err == nil {
		t.Error("expected error for non-matching tag")
	}
	user3 := &userdomain.User{}
	if err := checkApplicationTagAccess(app, user3); err == nil {
		t.Error("expected error for empty user tag")
	}
}

func TestIsCredentialError(t *testing.T) {
	if isCredentialError(nil) {
		t.Error("expected false for nil")
	}
	err := apperrors.NewUnauthorized("unauthorized")
	if !isCredentialError(err) {
		t.Error("expected true for unauthorized")
	}
	err2 := apperrors.NewNotFound("not found")
	if isCredentialError(err2) {
		t.Error("expected false for not found")
	}
}

func TestIsSupportedMfaType(t *testing.T) {
	if !isSupportedMfaType("totp") {
		t.Error("expected true for totp")
	}
	if !isSupportedMfaType("email") {
		t.Error("expected true for email")
	}
	if !isSupportedMfaType("sms") {
		t.Error("expected true for sms")
	}
	if !isSupportedMfaType("push") {
		t.Error("expected true for push")
	}
	if isSupportedMfaType("unknown") {
		t.Error("expected false for unknown")
	}
}

func TestEnabledVerifiedMfaMethods(t *testing.T) {
	user := &userdomain.User{
		MfaMethods: []userdomain.UserMfaMethod{
			{MethodType: "app", Enabled: true, Verified: true},
			{MethodType: "email", Enabled: true, Verified: false},
			{MethodType: "sms", Enabled: false, Verified: true},
		},
	}
	methods := enabledVerifiedMfaMethods(user)
	if len(methods) != 1 {
		t.Errorf("expected 1 method, got %d", len(methods))
	}
	if methods[0].MethodType != "app" {
		t.Errorf("expected app, got %s", methods[0].MethodType)
	}
}

func TestRequiredMfaSetupTypes(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			MfaPolicy: map[string]any{"required": true},
		},
	}
	user := &userdomain.User{MfaMethods: []userdomain.UserMfaMethod{}}
	types := requiredMfaSetupTypes(org, user)
	if len(types) != 1 || types[0] != "app" {
		t.Errorf("expected [app], got %v", types)
	}

	user2 := &userdomain.User{
		PreferredMfaType: "email",
		MfaMethods:       []userdomain.UserMfaMethod{},
	}
	types2 := requiredMfaSetupTypes(org, user2)
	if len(types2) != 1 || types2[0] != "email" {
		t.Errorf("expected [email], got %v", types2)
	}

	user3 := &userdomain.User{
		MfaMethods: []userdomain.UserMfaMethod{{MethodType: "app", Enabled: true, Verified: true}},
	}
	types3 := requiredMfaSetupTypes(org, user3)
	if types3 != nil {
		t.Errorf("expected nil when already has MFA, got %v", types3)
	}
}

func TestMfaRememberHours(t *testing.T) {
	if _, ok := mfaRememberHours(nil); ok {
		t.Error("expected false for nil")
	}
	if _, ok := mfaRememberHours(map[string]any{}); ok {
		t.Error("expected false for empty")
	}
	m := map[string]any{"mfaRememberHours": 24}
	if v, ok := mfaRememberHours(m); !ok || v != 24 {
		t.Errorf("expected 24, got %d, ok=%v", v, ok)
	}
	m2 := map[string]any{"mfaRememberInHours": int64(48)}
	if v, ok := mfaRememberHours(m2); !ok || v != 48 {
		t.Errorf("expected 48, got %d, ok=%v", v, ok)
	}
	m3 := map[string]any{"mfaRememberHours": "72"}
	if v, ok := mfaRememberHours(m3); !ok || v != 72 {
		t.Errorf("expected 72, got %d, ok=%v", v, ok)
	}
}

func TestResolveRequestedMfaType(t *testing.T) {
	methods := []userdomain.UserMfaMethod{{MethodType: "app"}, {MethodType: "email"}}
	if v := resolveRequestedMfaType("sms", "app", methods); v != "sms" {
		t.Errorf("got %q, want sms", v)
	}
	if v := resolveRequestedMfaType("", "app", methods); v != "app" {
		t.Errorf("got %q, want app", v)
	}
	if v := resolveRequestedMfaType("", "unknown", methods); v != "app" {
		t.Errorf("got %q, want app (first method)", v)
	}
	if v := resolveRequestedMfaType("", "", []userdomain.UserMfaMethod{}); v != "" {
		t.Errorf("got %q, want empty", v)
	}
}

func TestContainsMethodType(t *testing.T) {
	methods := []userdomain.UserMfaMethod{{MethodType: "app"}, {MethodType: "email"}}
	if !containsMethodType(methods, "app") {
		t.Error("expected true")
	}
	if containsMethodType(methods, "sms") {
		t.Error("expected false")
	}
}

func TestVerifyRecoveryCode(t *testing.T) {
	hashes := make([]string, 0, 3)
	for _, code := range []string{"abc", "def", "ghi"} {
		hash, err := platformcrypto.HashClientSecret(code)
		if err != nil {
			t.Fatalf("hash recovery code: %v", err)
		}
		hashes = append(hashes, hash)
	}
	user := &userdomain.User{RecoveryCodeHashes: hashes}
	if err := verifyRecoveryCode(user, "def"); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	if len(user.RecoveryCodeHashes) != 2 {
		t.Errorf("expected 2 codes after use, got %d", len(user.RecoveryCodeHashes))
	}
	if err := verifyRecoveryCode(user, "xyz"); err == nil {
		t.Error("expected error for invalid code")
	}
	if err := verifyRecoveryCode(user, "def"); err == nil {
		t.Error("expected error for consumed code")
	}
}

func TestMarkMfaMethodUsed(t *testing.T) {
	now := time.Now()
	user := &userdomain.User{
		MfaMethods: []userdomain.UserMfaMethod{
			{MethodType: "app", Enabled: true, Verified: true},
			{MethodType: "email", Enabled: true, Verified: true},
		},
	}
	markMfaMethodUsed(user, "app", now)
	if user.MfaMethods[0].LastUsedAt == nil || !user.MfaMethods[0].LastUsedAt.Equal(now) {
		t.Error("expected LastUsedAt to be set")
	}
	if user.MfaMethods[1].LastUsedAt != nil {
		t.Error("expected email LastUsedAt to remain nil")
	}
}
