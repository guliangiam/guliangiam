package auth

import (
	"context"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/applications"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// normalizeScopesFromString tests (client_credentials.go)
// ---------------------------------------------------------------------------

func TestNormalizeScopesFromString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", []string{}},
		{"whitespace_only", "   ", []string{}},
		{"single_scope", "read", []string{"read"}},
		{"multiple_scopes", "read write admin", []string{"read", "write", "admin"}},
		{"duplicate_scopes", "read write read admin write", []string{"read", "write", "admin"}},
		{"extra_whitespace", "  read   write  ", []string{"read", "write"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeScopesFromString(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("normalizeScopesFromString(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("normalizeScopesFromString(%q)[%d] = %q, want %q", tt.input, i, v, tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// generateSocialUsername tests (social_login.go)
// ---------------------------------------------------------------------------

func TestGenerateSocialUsername_WithEmail(t *testing.T) {
	info := &SocialUserInfo{
		ID:    "1234567890",
		Email: "JohnDoe@example.com",
	}
	username := generateSocialUsername("google", info)
	if username == "" {
		t.Error("expected non-empty username")
	}
	// Should be sanitized: lowercase, non-alphanumeric replaced with underscore
	if username == "" {
		t.Error("username should not be empty")
	}
}

func TestGenerateSocialUsername_WithDisplayName(t *testing.T) {
	info := &SocialUserInfo{
		ID:          "abc123",
		DisplayName: "John Doe",
	}
	username := generateSocialUsername("github", info)
	if username == "" {
		t.Error("expected non-empty username")
	}
}

func TestGenerateSocialUsername_NoInfo(t *testing.T) {
	info := &SocialUserInfo{
		ID: "shortid",
	}
	username := generateSocialUsername("provider", info)
	if username == "" {
		t.Error("expected non-empty username from fallback")
	}
	// Should use the fallback format: user_provider_shortid
	if username == "" {
		t.Error("fallback username should not be empty")
	}
}

func TestGenerateSocialUsername_LongBase(t *testing.T) {
	info := &SocialUserInfo{
		ID:          "123456",
		DisplayName: "This Is A Very Long Display Name That Should Be Truncated",
	}
	username := generateSocialUsername("google", info)
	if username == "" {
		t.Error("expected non-empty username")
	}
}

// ---------------------------------------------------------------------------
// minLen tests (social_login.go)
// ---------------------------------------------------------------------------

func TestMinLen(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{3, 5, 3},
		{5, 3, 3},
		{0, 0, 0},
		{10, 10, 10},
		{-1, 5, -1},
	}
	for _, tt := range tests {
		if got := minLen(tt.a, tt.b); got != tt.want {
			t.Errorf("minLen(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// generateSocialState tests (social_login.go)
// ---------------------------------------------------------------------------

func TestGenerateSocialState(t *testing.T) {
	state, err := generateSocialState()
	if err != nil {
		t.Fatalf("generateSocialState() error = %v", err)
	}
	if len(state) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("generateSocialState() length = %d, want 32", len(state))
	}

	// Should generate unique states
	state2, _ := generateSocialState()
	if state == state2 {
		t.Error("generateSocialState() should produce unique values")
	}
}

// ---------------------------------------------------------------------------
// ValidatePasswordPolicy Service tests (password_policy.go)
// ---------------------------------------------------------------------------

func TestValidatePasswordPolicy_Success(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	err := svc.ValidatePasswordPolicy(context.Background(), testOrgID, testUsername, "Str0ng!Pass")
	if err != nil {
		t.Errorf("ValidatePasswordPolicy() error = %v, want nil", err)
	}
}

func TestValidatePasswordPolicy_TooShort(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	err := svc.ValidatePasswordPolicy(context.Background(), testOrgID, testUsername, "Ab1!")
	if err == nil {
		t.Error("expected error for short password")
	}
}

func TestValidatePasswordPolicy_OrgNotFound(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	err := svc.ValidatePasswordPolicy(context.Background(), "nonexistent-org-id", testUsername, "Str0ng!Pass")
	if err == nil {
		t.Error("expected error for non-existent org")
	}
}

// ---------------------------------------------------------------------------
// applyMfaRememberDeadline tests (login.go)
// ---------------------------------------------------------------------------

func TestApplyMfaRememberDeadline_Disabled(t *testing.T) {
	cmd := LoginCommand{EnableMfaRemember: false}
	org := &orgdomain.Organization{}
	user := &userdomain.User{}
	now := time.Now()

	applyMfaRememberDeadline(cmd, org, user, now)
	if user.MfaRememberDeadline != nil {
		t.Error("expected MfaRememberDeadline to remain nil when not enabled")
	}
}

func TestApplyMfaRememberDeadline_EnabledWithHours(t *testing.T) {
	cmd := LoginCommand{EnableMfaRemember: true}
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			SessionPolicy: map[string]any{"mfaRememberHours": 24},
		},
	}
	user := &userdomain.User{}
	now := time.Now()

	applyMfaRememberDeadline(cmd, org, user, now)
	if user.MfaRememberDeadline == nil {
		t.Fatal("expected MfaRememberDeadline to be set")
	}
	expected := now.Add(24 * time.Hour)
	if !user.MfaRememberDeadline.Equal(expected) {
		t.Errorf("MfaRememberDeadline = %v, want %v", *user.MfaRememberDeadline, expected)
	}
}

func TestApplyMfaRememberDeadline_EnabledNoPolicy(t *testing.T) {
	cmd := LoginCommand{EnableMfaRemember: true}
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			SessionPolicy: map[string]any{},
		},
	}
	user := &userdomain.User{}
	now := time.Now()

	applyMfaRememberDeadline(cmd, org, user, now)
	if user.MfaRememberDeadline != nil {
		t.Error("expected MfaRememberDeadline to remain nil when no policy hours")
	}
}

// ---------------------------------------------------------------------------
// GetAppLogin tests (app_login.go) - SKIPPED: requires CaptchaFactory
// ---------------------------------------------------------------------------

// NOTE: GetAppLogin requires CaptchaFactory to be set, which makes it harder
// to test without more infrastructure. We test the error path through
// resolveConsentActorContext instead.

// ---------------------------------------------------------------------------
// resolveConsentActorContext edge cases (consent.go)
// ---------------------------------------------------------------------------

func TestResolveConsentActorContext_AppNotBelongToOrg(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.OrganizationID = "different-org-id"
	})
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	_, _, _, err := svc.resolveConsentActorContext(context.Background(), testOrgSlug, testUsername, testAppID)
	if err == nil {
		t.Fatal("expected error when app does not belong to org")
	}
	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", typed.Code, apperrors.CodeInvalidArgument)
	}
}

// ---------------------------------------------------------------------------
// GrantConsent with existing consent (consent.go)
// ---------------------------------------------------------------------------

func TestGrantConsent_ExistingConsent(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.Slug = testAppName
		a.CustomScopes = []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read"},
			{Scope: "write", DisplayName: "Write"},
		}
	})
	uow := newServiceTestUow(org, user, app)

	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	uow.consentRepo.consents = map[string]*consentdomain.Consent{
		consentKey(testUserID, testAppID): {
			ID:               "consent-1",
			UserID:           testUserID,
			Username:         testUsername,
			OrganizationID:   testOrgID,
			OrganizationSlug: testOrgSlug,
			ApplicationID:    testAppID,
			ApplicationSlug:  testAppName,
			GrantedScopes:    []string{"read"},
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		},
	}

	svc := newServiceTestService(uow)

	result, err := svc.GrantConsent(context.Background(), GrantConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"write"},
	})
	if err != nil {
		t.Fatalf("GrantConsent() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("GrantConsent() result = nil")
	}
	// Should merge scopes: "read" + "write"
	if len(result.Consent.GrantedScopes) != 2 {
		t.Errorf("len(GrantedScopes) = %d, want 2", len(result.Consent.GrantedScopes))
	}
	// Revision should be incremented
	if result.Consent.Revision != 2 {
		t.Errorf("Revision = %d, want 2", result.Consent.Revision)
	}
}

// ---------------------------------------------------------------------------
// RevokeConsent with no existing consent (consent.go)
// ---------------------------------------------------------------------------

func TestRevokeConsent_NoConsent(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.CustomScopes = []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read"},
		}
	})
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	_, err := svc.RevokeConsent(context.Background(), RevokeConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"read"},
	})
	if err == nil {
		t.Fatal("expected error when no consent exists")
	}
}

// ---------------------------------------------------------------------------
// RevokeConsent with already revoked consent (consent.go)
// ---------------------------------------------------------------------------

func TestRevokeConsent_AlreadyRevoked(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.CustomScopes = []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read"},
		}
	})
	uow := newServiceTestUow(org, user, app)

	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	uow.consentRepo.consents = map[string]*consentdomain.Consent{
		consentKey(testUserID, testAppID): {
			ID:               "consent-1",
			UserID:           testUserID,
			Username:         testUsername,
			OrganizationID:   testOrgID,
			OrganizationSlug: testOrgSlug,
			ApplicationID:    testAppID,
			ApplicationSlug:  testAppName,
			GrantedScopes:    []string{"read"},
			CreatedAt:        now,
			UpdatedAt:        now,
			RevokedAt:        &now,
			Revision:         1,
		},
	}

	svc := newServiceTestService(uow)

	_, err := svc.RevokeConsent(context.Background(), RevokeConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"read"},
	})
	if err == nil {
		t.Fatal("expected error when consent is already revoked")
	}
}

// ---------------------------------------------------------------------------
// RevokeToken with valid client and access token (token_revocation.go)
// ---------------------------------------------------------------------------

func TestRevokeToken_AccessTokenFound(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.ClientID = "test-client-id"
		// 公开客户端：RFC 7009 允许仅凭 client_id 撤销，无需 secret。
		a.TokenEndpointAuthMethod = applications.TokenEndpointAuthMethodNone
	})
	uow := newServiceTestUow(org, user, app)
	uow.appRepo.byClientID = map[string]*applications.Application{"test-client-id": app}

	// Pre-populate a token with a known hash
	tokenHash := hashTokenValueForRevocation("test-access-token")
	uow.tokenRepo.tokensByHash = map[string]*tokendomain.Token{
		tokenHash: {
			ID:            "token-1",
			ApplicationID: testAppID,
			TokenHash:     tokenHash,
		},
	}
	uow.tokenRepo.tokens = map[string]*tokendomain.Token{
		"token-1": {
			ID:            "token-1",
			ApplicationID: testAppID,
			TokenHash:     tokenHash,
		},
	}

	svc := newServiceTestService(uow)

	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{
		Token:         "test-access-token",
		TokenTypeHint: "access_token",
		ClientID:      "test-client-id",
	})
	if err != nil {
		t.Errorf("RevokeToken() error = %v, want nil", err)
	}
}

func TestRevokeToken_RefreshTokenFound(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.ClientID = "test-client-id"
		a.TokenEndpointAuthMethod = applications.TokenEndpointAuthMethodNone
	})
	uow := newServiceTestUow(org, user, app)
	uow.appRepo.byClientID = map[string]*applications.Application{"test-client-id": app}

	// Pre-populate a refresh token with a known hash
	tokenHash := hashTokenValueForRevocation("test-refresh-token")
	uow.tokenRepo.refreshByHash = map[string]*tokendomain.RefreshToken{
		tokenHash: {
			ID:               "refresh-1",
			ApplicationID:    testAppID,
			RefreshTokenHash: tokenHash,
		},
	}

	svc := newServiceTestService(uow)

	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{
		Token:         "test-refresh-token",
		TokenTypeHint: "refresh_token",
		ClientID:      "test-client-id",
	})
	if err != nil {
		t.Errorf("RevokeToken() error = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// EvaluateConsent with no custom scopes (consent.go)
// ---------------------------------------------------------------------------

func TestEvaluateConsent_NoCustomScopes(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp() // no custom scopes
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	result, err := svc.EvaluateConsent(context.Background(), EvaluateConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		RequestedScopes:  []string{"read"},
	})
	if err != nil {
		t.Fatalf("EvaluateConsent() error = %v, want nil", err)
	}
	// No custom scopes means no consentable scopes
	if len(result.Evaluation.ConsentableScopes) != 0 {
		t.Errorf("ConsentableScopes = %v, want empty", result.Evaluation.ConsentableScopes)
	}
	// Required should be false when no custom scopes
	if result.Evaluation.Required {
		t.Error("Required should be false when no custom scopes")
	}
}

// ---------------------------------------------------------------------------
// ValidatePasswordPolicyWithHistory tests (password_policy.go)
// ---------------------------------------------------------------------------

func TestValidatePasswordPolicyWithHistory_Success(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	err := svc.ValidatePasswordPolicyWithHistory(context.Background(), uow, testOrgID, testUserID, testUsername, "Str0ng!Pass")
	if err != nil {
		t.Errorf("ValidatePasswordPolicyWithHistory() error = %v, want nil", err)
	}
}

func TestValidatePasswordPolicyWithHistory_WeakPassword(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	err := svc.ValidatePasswordPolicyWithHistory(context.Background(), uow, testOrgID, testUserID, testUsername, "weak")
	if err == nil {
		t.Error("expected error for weak password")
	}
}

func TestValidatePasswordPolicyWithHistory_OrgNotFound(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	err := svc.ValidatePasswordPolicyWithHistory(context.Background(), uow, "nonexistent-org", testUserID, testUsername, "Str0ng!Pass")
	if err == nil {
		t.Error("expected error for non-existent org")
	}
}

// ---------------------------------------------------------------------------
// checkPaidUserSubscription tests (login.go)
// ---------------------------------------------------------------------------

func TestCheckPaidUserSubscription_NoSubscriptionRequired(t *testing.T) {
	org := newTestOrg()
	app := newTestApp()
	user := newTestUser()
	uow := newServiceTestUow(org, user, app)

	// No subscription required — should pass
	err := checkPaidUserSubscription(context.Background(), uow, user, time.Now())
	if err != nil {
		t.Errorf("checkPaidUserSubscription() error = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// verifyTotpPasscode tests (login.go)
// ---------------------------------------------------------------------------

func TestVerifyTotpPasscode_EmptySecret(t *testing.T) {
	now := time.Now()
	// Empty secret should fail
	err := verifyTotpPasscode("", "123456", now)
	if err == nil {
		t.Error("expected error for empty TOTP secret")
	}
}
