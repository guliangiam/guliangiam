package auth

import (
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

func TestToAccountContext_AllFields(t *testing.T) {
	org := &orgdomain.Organization{
		ID:          "org-1",
		Slug:        "acme",
		DisplayName: "Acme Corp",
		LogoURL:     "https://acme.com/logo.png",
		FaviconURL:  "https://acme.com/favicon.ico",
	}
	user := &userdomain.User{
		Username: "alice",
		IsAdmin:  true,
		Profile: userdomain.UserProfile{
			DisplayName: "Alice Smith",
			AvatarURL:   "https://acme.com/avatars/alice.png",
		},
	}
	accessToken := "at-12345"

	ctx := toAccountContext(org, user, accessToken)

	if ctx.Account.Owner != "acme" {
		t.Errorf("Account.Owner = %q, want %q", ctx.Account.Owner, "acme")
	}
	if ctx.Account.Name != "alice" {
		t.Errorf("Account.Name = %q, want %q", ctx.Account.Name, "alice")
	}
	if ctx.Account.DisplayName != "Alice Smith" {
		t.Errorf("Account.DisplayName = %q, want %q", ctx.Account.DisplayName, "Alice Smith")
	}
	if ctx.Account.Avatar != "https://acme.com/avatars/alice.png" {
		t.Errorf("Account.Avatar = %q, want avatar URL", ctx.Account.Avatar)
	}
	if !ctx.Account.IsAdmin {
		t.Error("Account.IsAdmin = false, want true")
	}
	if ctx.Organization.ID != "org-1" {
		t.Errorf("Organization.ID = %q, want %q", ctx.Organization.ID, "org-1")
	}
	if ctx.Organization.Name != "acme" {
		t.Errorf("Organization.Name = %q, want %q", ctx.Organization.Name, "acme")
	}
	if ctx.Organization.DisplayName != "Acme Corp" {
		t.Errorf("Organization.DisplayName = %q, want %q", ctx.Organization.DisplayName, "Acme Corp")
	}
	if ctx.Organization.LogoUrl != "https://acme.com/logo.png" {
		t.Errorf("Organization.LogoUrl = %q, want logo URL", ctx.Organization.LogoUrl)
	}
	if ctx.Organization.FaviconUrl != "https://acme.com/favicon.ico" {
		t.Errorf("Organization.FaviconUrl = %q, want favicon URL", ctx.Organization.FaviconUrl)
	}
	if ctx.AccessToken != "at-12345" {
		t.Errorf("AccessToken = %q, want %q", ctx.AccessToken, "at-12345")
	}
}

func TestToAccountContext_EmptyFields(t *testing.T) {
	org := &orgdomain.Organization{
		ID:   "org-2",
		Slug: "minimal",
	}
	user := &userdomain.User{
		Username: "bob",
		Profile:  userdomain.UserProfile{},
	}

	ctx := toAccountContext(org, user, "")

	if ctx.Account.DisplayName != "" {
		t.Errorf("Account.DisplayName = %q, want empty", ctx.Account.DisplayName)
	}
	if ctx.Account.Avatar != "" {
		t.Errorf("Account.Avatar = %q, want empty", ctx.Account.Avatar)
	}
	if ctx.AccessToken != "" {
		t.Errorf("AccessToken = %q, want empty", ctx.AccessToken)
	}
}

func TestToSuccessfulLoginResponse_AllFields(t *testing.T) {
	account := AccountContext{
		Account: SessionAccount{
			Owner:   "acme",
			Name:    "alice",
			IsAdmin: true,
		},
		Organization: SessionOrganization{
			ID:   "org-1",
			Name: "acme",
		},
		AccessToken: "at-12345",
	}

	resp := toSuccessfulLoginResponse(account)

	if resp.Status != LoginStatusSuccess {
		t.Errorf("Status = %q, want %q", resp.Status, LoginStatusSuccess)
	}
	if resp.Account == nil {
		t.Fatal("Account = nil, want non-nil")
	}
	if resp.Account.Name != "alice" {
		t.Errorf("Account.Name = %q, want %q", resp.Account.Name, "alice")
	}
	if resp.Organization == nil {
		t.Fatal("Organization = nil, want non-nil")
	}
	if resp.Organization.Name != "acme" {
		t.Errorf("Organization.Name = %q, want %q", resp.Organization.Name, "acme")
	}
	if resp.AccessToken != "at-12345" {
		t.Errorf("AccessToken = %q, want %q", resp.AccessToken, "at-12345")
	}
}

func TestToMfaRequiredLoginResponse_SingleMethod(t *testing.T) {
	user := &userdomain.User{
		Profile: userdomain.UserProfile{
			Email: "alice@example.com",
		},
	}
	methods := []userdomain.UserMfaMethod{
		{ID: "mfa-1", MethodType: "totp", DisplayName: "Authenticator"},
	}
	expiresAt := time.Date(2026, 6, 12, 10, 10, 0, 0, time.UTC)

	resp := toMfaRequiredLoginResponse(user, "totp", methods, "challenge-1", "pending", expiresAt)

	if resp.Status != LoginStatusMfaRequired {
		t.Errorf("Status = %q, want %q", resp.Status, LoginStatusMfaRequired)
	}
	if resp.ChallengeID != "challenge-1" {
		t.Errorf("ChallengeID = %q, want %q", resp.ChallengeID, "challenge-1")
	}
	if resp.NextMfa != "totp" {
		t.Errorf("NextMfa = %q, want %q", resp.NextMfa, "totp")
	}
	if len(resp.RequiredMfa) != 1 || resp.RequiredMfa[0] != "totp" {
		t.Errorf("RequiredMfa = %v, want [totp]", resp.RequiredMfa)
	}
	if len(resp.AvailableMfaMethods) != 1 {
		t.Fatalf("AvailableMfaMethods len = %d, want 1", len(resp.AvailableMfaMethods))
	}
	if resp.AvailableMfaMethods[0].ID != "mfa-1" {
		t.Errorf("AvailableMfaMethods[0].ID = %q, want %q", resp.AvailableMfaMethods[0].ID, "mfa-1")
	}
	if resp.ChallengeExpiresAt == nil {
		t.Fatal("ChallengeExpiresAt = nil, want non-nil")
	}
}

func TestToMfaRequiredLoginResponse_MultipleMethods(t *testing.T) {
	user := &userdomain.User{
		Profile: userdomain.UserProfile{
			Email: "alice@example.com",
			Phone: "+1234567890",
		},
	}
	methods := []userdomain.UserMfaMethod{
		{ID: "mfa-1", MethodType: "totp", DisplayName: "Authenticator"},
		{ID: "mfa-2", MethodType: "email", DisplayName: "Email"},
		{ID: "mfa-3", MethodType: "sms", DisplayName: "SMS"},
	}
	expiresAt := time.Date(2026, 6, 12, 10, 10, 0, 0, time.UTC)

	resp := toMfaRequiredLoginResponse(user, "email", methods, "challenge-2", "pending", expiresAt)

	if resp.NextMfa != "email" {
		t.Errorf("NextMfa = %q, want %q (preferred)", resp.NextMfa, "email")
	}
	if len(resp.RequiredMfa) != 3 {
		t.Errorf("RequiredMfa len = %d, want 3", len(resp.RequiredMfa))
	}
	if len(resp.AvailableMfaMethods) != 3 {
		t.Errorf("AvailableMfaMethods len = %d, want 3", len(resp.AvailableMfaMethods))
	}
}

func TestToMfaRequiredLoginResponse_DuplicateMethodTypes(t *testing.T) {
	user := &userdomain.User{
		Profile: userdomain.UserProfile{
			Email: "alice@example.com",
		},
	}
	methods := []userdomain.UserMfaMethod{
		{ID: "mfa-1", MethodType: "totp", DisplayName: "Authenticator 1"},
		{ID: "mfa-2", MethodType: "totp", DisplayName: "Authenticator 2"},
	}
	expiresAt := time.Date(2026, 6, 12, 10, 10, 0, 0, time.UTC)

	resp := toMfaRequiredLoginResponse(user, "", methods, "challenge-3", "pending", expiresAt)

	// Duplicate method types should be deduplicated in RequiredMfa
	if len(resp.RequiredMfa) != 1 {
		t.Errorf("RequiredMfa len = %d, want 1 (deduplicated)", len(resp.RequiredMfa))
	}
	// But all methods should appear in AvailableMfaMethods
	if len(resp.AvailableMfaMethods) != 2 {
		t.Errorf("AvailableMfaMethods len = %d, want 2", len(resp.AvailableMfaMethods))
	}
}

func TestToMfaSetupRequiredLoginResponse_Extended(t *testing.T) {
	expiresAt := time.Date(2026, 6, 12, 10, 10, 0, 0, time.UTC)
	requiredMfa := []string{"totp", "email"}

	resp := toMfaSetupRequiredLoginResponse(requiredMfa, "challenge-setup-1", expiresAt)

	if resp.Status != LoginStatusMfaSetup {
		t.Errorf("Status = %q, want %q", resp.Status, LoginStatusMfaSetup)
	}
	if resp.ChallengeID != "challenge-setup-1" {
		t.Errorf("ChallengeID = %q, want %q", resp.ChallengeID, "challenge-setup-1")
	}
	if len(resp.RequiredMfa) != 2 {
		t.Errorf("RequiredMfa len = %d, want 2", len(resp.RequiredMfa))
	}
	if resp.ChallengeExpiresAt == nil {
		t.Fatal("ChallengeExpiresAt = nil, want non-nil")
	}
}

func TestLoginMfaMethodTarget_NilUser(t *testing.T) {
	target, cc := loginMfaMethodTarget(nil, "email")
	if target != "" || cc != "" {
		t.Errorf("expected empty for nil user, got (%q, %q)", target, cc)
	}
}

func TestLoginMfaMethodTarget_EmailMethod(t *testing.T) {
	user := &userdomain.User{Profile: userdomain.UserProfile{Email: "alice@example.com"}}
	target, cc := loginMfaMethodTarget(user, "email")
	if target != "alice@example.com" {
		t.Errorf("target = %q, want %q", target, "alice@example.com")
	}
	if cc != "" {
		t.Errorf("countryCode = %q, want empty for email", cc)
	}
}

func TestLoginMfaMethodTarget_SmsMethod(t *testing.T) {
	user := &userdomain.User{Profile: userdomain.UserProfile{Phone: "+1234567890", CountryCode: "+1"}}
	target, cc := loginMfaMethodTarget(user, "sms")
	if target != "+1234567890" {
		t.Errorf("target = %q, want %q", target, "+1234567890")
	}
	if cc != "+1" {
		t.Errorf("countryCode = %q, want %q", cc, "+1")
	}
}

// ISO alpha-2 is not a calling code: the login UI concatenates `countryCode`
// with the phone, so leaking `CN` would produce `CN13800138000`.
func TestLoginMfaMethodTarget_SmsMethod_ISOCountryIsNotDialCode(t *testing.T) {
	user := &userdomain.User{Profile: userdomain.UserProfile{Phone: "+8613800138000", CountryCode: "CN"}}
	target, cc := loginMfaMethodTarget(user, "sms")
	if target != "+8613800138000" {
		t.Errorf("target = %q, want %q", target, "+8613800138000")
	}
	if cc != "" {
		t.Errorf("countryCode = %q, want empty for ISO alpha-2", cc)
	}
}

func TestLoginMfaMethodTarget_RadiusMethod(t *testing.T) {
	user := &userdomain.User{MfaRadiusUsername: "radiususer"}
	target, cc := loginMfaMethodTarget(user, "radius")
	if target != "radiususer" {
		t.Errorf("target = %q, want %q", target, "radiususer")
	}
	if cc != "" {
		t.Errorf("countryCode = %q, want empty for radius", cc)
	}
}

func TestLoginMfaMethodTarget_PushMethod(t *testing.T) {
	user := &userdomain.User{MfaPushReceiver: "push-user-123"}
	target, cc := loginMfaMethodTarget(user, "push")
	if target != "push-user-123" {
		t.Errorf("target = %q, want %q", target, "push-user-123")
	}
	if cc != "" {
		t.Errorf("countryCode = %q, want empty for push", cc)
	}
}

func TestLoginMfaMethodTarget_UnknownMethod(t *testing.T) {
	user := &userdomain.User{Profile: userdomain.UserProfile{Email: "test@test.com"}}
	target, cc := loginMfaMethodTarget(user, "unknown")
	if target != "" || cc != "" {
		t.Errorf("expected empty for unknown method, got (%q, %q)", target, cc)
	}
}
