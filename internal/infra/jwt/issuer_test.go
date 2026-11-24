package jwt

import (
	"testing"
	"time"

	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

func newTestIssuer(t *testing.T) *Issuer {
	t.Helper()
	mgr := platformjwks.NewManager(2048, nil)
	return NewIssuer(mgr)
}

func TestIssuer_IssueAccessToken(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:         "https://auth.example.com",
		Subject:        "user-1",
		Audience:       []string{"app-1"},
		TTL:            time.Hour,
		JWTID:          "jti-1",
		Scope:          []string{"openid", "profile"},
		ClientID:       "app-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Resource:       []string{"https://api.example.com"},
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	parts := splitToken(token)
	if len(parts) != 3 {
		t.Errorf("expected 3 JWT parts, got %d", len(parts))
	}
}

func TestIssuer_IssueIDToken(t *testing.T) {
	issuer := newTestIssuer(t)
	emailVerified := true
	phoneVerified := false
	token, err := issuer.IssueIDToken(IssueIDTokenParams{
		Issuer:             "https://auth.example.com",
		Subject:            "user-1",
		Audience:           []string{"app-1"},
		Nonce:              "nonce-123",
		AccessTokenHash:    "ath",
		CodeHash:           "ch",
		AuthenticationTime: time.Now(),
		JWTID:              "jti-2",
		Name:               "Alice Smith",
		Username:           "alice",
		GivenName:          "Alice",
		FamilyName:         "Smith",
		Email:              "alice@example.com",
		EmailVerified:      &emailVerified,
		Phone:              "+1234567890",
		PhoneVerified:      &phoneVerified,
	})
	if err != nil {
		t.Fatalf("IssueIDToken() error = %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestIssuer_ValidateToken_Valid(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      time.Hour,
		JWTID:    "jti-1",
		Scope:    []string{"openid"},
		ClientID: "app-1",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	claims, err := issuer.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims["sub"] != "user-1" {
		t.Errorf("sub = %v, want user-1", claims["sub"])
	}
	if claims["iss"] != "https://auth.example.com" {
		t.Errorf("iss = %v, want https://auth.example.com", claims["iss"])
	}
}

func TestIssuer_ValidateToken_InvalidFormat(t *testing.T) {
	issuer := newTestIssuer(t)
	_, err := issuer.ValidateToken("not.a.valid.jwt.token")
	if err == nil {
		t.Error("expected error for invalid token format")
	}
}

func TestIssuer_ValidateToken_EmptyString(t *testing.T) {
	issuer := newTestIssuer(t)
	_, err := issuer.ValidateToken("")
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestIssuer_ValidateToken_TwoParts(t *testing.T) {
	issuer := newTestIssuer(t)
	_, err := issuer.ValidateToken("part1.part2")
	if err == nil {
		t.Error("expected error for two-part token")
	}
}

func TestIssuer_ValidateToken_InvalidSignature(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      time.Hour,
		JWTID:    "jti-1",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	parts := splitToken(token)
	tampered := parts[0] + "." + parts[1] + ".AAAAAAAAAAAAAAAAAAA"
	_, err = issuer.ValidateToken(tampered)
	if err == nil {
		t.Error("expected error for tampered signature")
	}
}

func TestIssuer_ValidateToken_Expired(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      -time.Hour,
		JWTID:    "jti-exp",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	_, err = issuer.ValidateToken(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestIssuer_ValidateTokenWithConfig_IssuerCheck(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      time.Hour,
		JWTID:    "jti-iss",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	claims, err := issuer.ValidateTokenWithConfig(token, ValidationConfig{
		ExpectedIssuer: "https://auth.example.com",
	})
	if err != nil {
		t.Fatalf("ValidateTokenWithConfig() error = %v", err)
	}
	if claims["sub"] != "user-1" {
		t.Errorf("sub = %v, want user-1", claims["sub"])
	}

	_, err = issuer.ValidateTokenWithConfig(token, ValidationConfig{
		ExpectedIssuer: "https://wrong.example.com",
	})
	if err == nil {
		t.Error("expected error for wrong issuer")
	}
}

func TestIssuer_ValidateTokenWithConfig_AudienceCheck(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      time.Hour,
		JWTID:    "jti-aud",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	claims, err := issuer.ValidateTokenWithConfig(token, ValidationConfig{
		ExpectedAudience: "app-1",
	})
	if err != nil {
		t.Fatalf("ValidateTokenWithConfig() error = %v", err)
	}
	aud, ok := claims["aud"].([]interface{})
	if !ok || len(aud) == 0 || aud[0].(string) != "app-1" {
		t.Errorf("aud = %v, want [app-1]", claims["aud"])
	}

	_, err = issuer.ValidateTokenWithConfig(token, ValidationConfig{
		ExpectedAudience: "wrong-app",
	})
	if err == nil {
		t.Error("expected error for wrong audience")
	}
}

func TestIssuer_ValidateTokenWithConfig_BothChecks(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      time.Hour,
		JWTID:    "jti-both",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	_, err = issuer.ValidateTokenWithConfig(token, ValidationConfig{
		ExpectedIssuer:   "https://auth.example.com",
		ExpectedAudience: "app-1",
	})
	if err != nil {
		t.Fatalf("ValidateTokenWithConfig() error = %v", err)
	}
}

func TestNewIssuer(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	iss := NewIssuer(mgr)
	if iss == nil {
		t.Error("expected non-nil issuer")
	}
	if iss.jwks != mgr {
		t.Error("jwks manager not set correctly")
	}
}

func splitToken(token string) []string {
	count := 0
	for _, c := range token {
		if c == '.' {
			count++
		}
	}
	if count != 2 {
		return []string{token}
	}
	parts := make([]string, 0, 3)
	start := 0
	for i, c := range token {
		if c == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}
