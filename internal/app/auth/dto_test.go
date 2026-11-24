package auth

import (
	"testing"
	"time"
)

func TestLoginResponse_Status(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"success", LoginStatusSuccess},
		{"mfa_required", LoginStatusMfaRequired},
		{"mfa_setup", LoginStatusMfaSetup},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := LoginResponse{Status: tt.status}
			if resp.Status != tt.status {
				t.Errorf("Status = %q, want %q", resp.Status, tt.status)
			}
		})
	}
}

func TestLoginResponse_WithAccount(t *testing.T) {
	resp := LoginResponse{
		Status: LoginStatusSuccess,
		Account: &SessionAccount{
			Owner:       "acme",
			Name:        "alice",
			DisplayName: "Alice Smith",
			Avatar:      "https://example.com/avatar.png",
			IsAdmin:     true,
		},
	}
	if resp.Account == nil {
		t.Fatal("Account should not be nil")
	}
	if resp.Account.Name != "alice" {
		t.Errorf("Account.Name = %q, want %q", resp.Account.Name, "alice")
	}
	if !resp.Account.IsAdmin {
		t.Error("Account.IsAdmin should be true")
	}
}

func TestLoginResponse_MfaRequiredFields(t *testing.T) {
	resp := LoginResponse{
		Status:      LoginStatusMfaRequired,
		ChallengeID: "challenge-1",
		AvailableMfaMethods: []LoginMfaMethod{
			{ID: "mfa-1", MethodType: "totp", DisplayName: "Authenticator"},
			{ID: "mfa-2", MethodType: "sms", Target: "+1234567890"},
		},
	}
	if resp.ChallengeID != "challenge-1" {
		t.Errorf("ChallengeID = %q, want %q", resp.ChallengeID, "challenge-1")
	}
	if len(resp.AvailableMfaMethods) != 2 {
		t.Fatalf("len(AvailableMfaMethods) = %d, want 2", len(resp.AvailableMfaMethods))
	}
	if resp.AvailableMfaMethods[0].MethodType != "totp" {
		t.Errorf("Method[0].MethodType = %q, want %q", resp.AvailableMfaMethods[0].MethodType, "totp")
	}
}

func TestLoginResponse_MfaSetupFields(t *testing.T) {
	resp := LoginResponse{
		Status:      LoginStatusMfaSetup,
		ChallengeID: "challenge-setup-1",
		RequiredMfa: []string{"app", "email"},
	}
	if len(resp.RequiredMfa) != 2 {
		t.Fatalf("len(RequiredMfa) = %d, want 2", len(resp.RequiredMfa))
	}
}

func TestSessionAccount_Fields(t *testing.T) {
	account := SessionAccount{
		Owner:       "acme",
		Name:        "alice",
		DisplayName: "Alice",
		Avatar:      "https://example.com/avatar.png",
		IsAdmin:     false,
	}
	if account.Owner != "acme" {
		t.Errorf("Owner = %q, want %q", account.Owner, "acme")
	}
	if account.IsAdmin {
		t.Error("IsAdmin should be false")
	}
}

func TestSessionOrganization_Fields(t *testing.T) {
	org := SessionOrganization{
		ID:          "org-1",
		Name:        "acme",
		DisplayName: "ACME Corp",
		LogoUrl:     "https://example.com/logo.png",
		NavItems:    []string{"dashboard", "users"},
	}
	if org.ID != "org-1" {
		t.Errorf("ID = %q, want %q", org.ID, "org-1")
	}
	if len(org.NavItems) != 2 {
		t.Errorf("len(NavItems) = %d, want 2", len(org.NavItems))
	}
}

func TestAccountContext_Fields(t *testing.T) {
	ctx := AccountContext{
		Account: SessionAccount{
			Owner: "acme",
			Name:  "alice",
		},
		Organization: SessionOrganization{
			Name: "acme",
		},
		AccessToken: "token-123",
	}
	if ctx.AccessToken != "token-123" {
		t.Errorf("AccessToken = %q, want %q", ctx.AccessToken, "token-123")
	}
}

func TestLoginMfaMethod_Fields(t *testing.T) {
	method := LoginMfaMethod{
		ID:          "mfa-1",
		MethodType:  "sms",
		DisplayName: "Phone",
		Target:      "+1234567890",
		CountryCode: "+1",
	}
	if method.MethodType != "sms" {
		t.Errorf("MethodType = %q, want %q", method.MethodType, "sms")
	}
	if method.CountryCode != "+1" {
		t.Errorf("CountryCode = %q, want %q", method.CountryCode, "+1")
	}
}

func TestExchangeAuthorizationCodeResponse_Fields(t *testing.T) {
	resp := ExchangeAuthorizationCodeResponse{
		Status:       "success",
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		Scope:        "openid profile email",
	}
	if resp.Status != "success" {
		t.Errorf("Status = %q, want %q", resp.Status, "success")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want %q", resp.TokenType, "Bearer")
	}
	if resp.ExpiresIn <= 0 {
		t.Errorf("ExpiresIn = %d, want positive", resp.ExpiresIn)
	}
}

func TestConsentDetail_Fields(t *testing.T) {
	now := time.Now()
	detail := ConsentDetail{
		ID:               "consent-1",
		UserID:           "user-1",
		Username:         "alice",
		OrganizationID:   "org-1",
		OrganizationSlug: "acme",
		ApplicationID:    "app-1",
		ApplicationSlug:  "myapp",
		GrantedScopes:    []string{"read", "write"},
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         1,
	}
	if detail.ID != "consent-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "consent-1")
	}
	if len(detail.GrantedScopes) != 2 {
		t.Errorf("len(GrantedScopes) = %d, want 2", len(detail.GrantedScopes))
	}
}

func TestConsentEvaluationDetail_Fields(t *testing.T) {
	detail := ConsentEvaluationDetail{
		ApplicationID:     "app-1",
		ApplicationSlug:   "myapp",
		RequestedScopes:   []string{"read", "write", "admin"},
		ConsentableScopes: []string{"read", "write"},
		GrantedScopes:     []string{"read"},
		MissingScopes:     []string{"write"},
		Required:          true,
	}
	if !detail.Required {
		t.Error("Required should be true")
	}
	if len(detail.MissingScopes) != 1 {
		t.Errorf("len(MissingScopes) = %d, want 1", len(detail.MissingScopes))
	}
}

func TestAuthorizeResponse_Fields(t *testing.T) {
	resp := AuthorizeResponse{
		Status: "consent_required",
		Evaluation: &ConsentEvaluationDetail{
			ApplicationID: "app-1",
			Required:      true,
		},
	}
	if resp.Status != "consent_required" {
		t.Errorf("Status = %q, want %q", resp.Status, "consent_required")
	}
	if resp.Evaluation == nil {
		t.Fatal("Evaluation should not be nil")
	}
}

func TestLoginChallengeResponse_Fields(t *testing.T) {
	now := time.Now()
	resp := LoginChallengeResponse{
		ID:         "challenge-1",
		State:      "pending",
		MethodType: "totp",
		ExpiresAt:  now.Add(10 * time.Minute),
	}
	if resp.ID != "challenge-1" {
		t.Errorf("ID = %q, want %q", resp.ID, "challenge-1")
	}
	if resp.State != "pending" {
		t.Errorf("State = %q, want %q", resp.State, "pending")
	}
}
