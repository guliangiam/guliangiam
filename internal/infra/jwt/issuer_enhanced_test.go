package jwt

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

func TestIssuer_IssueAccessToken_AllFields(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:         "https://auth.example.com",
		Subject:        "user-1",
		Audience:       []string{"app-1", "app-2"},
		TTL:            30 * time.Minute,
		JWTID:          "jti-fields",
		Scope:          []string{"openid", "profile", "email"},
		ClientID:       "app-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Resource:       []string{"https://api.example.com", "https://api2.example.com"},
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		t.Fatalf("parse claims: %v", err)
	}

	if claims["sub"] != "user-1" {
		t.Errorf("sub = %v, want user-1", claims["sub"])
	}
	if claims["iss"] != "https://auth.example.com" {
		t.Errorf("iss = %v, want https://auth.example.com", claims["iss"])
	}
	if claims["scope"] != "openid profile email" {
		t.Errorf("scope = %v, want openid profile email", claims["scope"])
	}
	if claims["client_id"] != "app-1" {
		t.Errorf("client_id = %v, want app-1", claims["client_id"])
	}
	if claims["org_id"] != "org-1" {
		t.Errorf("org_id = %v, want org-1", claims["org_id"])
	}
	if claims["username"] != "alice" {
		t.Errorf("username = %v, want alice", claims["username"])
	}
	if claims["jti"] != "jti-fields" {
		t.Errorf("jti = %v, want jti-fields", claims["jti"])
	}

	aud, ok := claims["aud"].([]interface{})
	if !ok || len(aud) != 2 {
		t.Errorf("aud = %v, want [app-1, app-2]", claims["aud"])
	}
}

func TestIssuer_IssueIDToken_AllFields(t *testing.T) {
	issuer := newTestIssuer(t)
	emailVerified := true
	phoneVerified := true
	token, err := issuer.IssueIDToken(IssueIDTokenParams{
		Issuer:             "https://auth.example.com",
		Subject:            "user-2",
		Audience:           []string{"app-1"},
		Nonce:              "nonce-abc",
		AccessTokenHash:    "ath-value",
		CodeHash:           "ch-value",
		AuthenticationTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		JWTID:              "jti-id-1",
		Name:               "Bob Jones",
		Username:           "bob",
		GivenName:          "Bob",
		FamilyName:         "Jones",
		MiddleName:         "M",
		Nickname:           "bj",
		Email:              "bob@example.com",
		EmailVerified:      &emailVerified,
		Phone:              "+1234567890",
		PhoneVerified:      &phoneVerified,
		Picture:            "https://img.example.com/bob.jpg",
		Organization:       "org-1",
	})
	if err != nil {
		t.Fatalf("IssueIDToken() error = %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		t.Fatalf("parse claims: %v", err)
	}

	if claims["sub"] != "user-2" {
		t.Errorf("sub = %v, want user-2", claims["sub"])
	}
	if claims["nonce"] != "nonce-abc" {
		t.Errorf("nonce = %v, want nonce-abc", claims["nonce"])
	}
	if claims["name"] != "Bob Jones" {
		t.Errorf("name = %v, want Bob Jones", claims["name"])
	}
	if claims["email"] != "bob@example.com" {
		t.Errorf("email = %v, want bob@example.com", claims["email"])
	}
	if claims["picture"] != "https://img.example.com/bob.jpg" {
		t.Errorf("picture = %v, want https://img.example.com/bob.jpg", claims["picture"])
	}
	if claims["org"] != "org-1" {
		t.Errorf("org = %v, want org-1", claims["org"])
	}
}

func TestIssuer_ValidateToken_TamperedPayload(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      time.Hour,
		JWTID:    "jti-tamper",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	parts := strings.Split(token, ".")
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	claims["sub"] = "user-hacked"
	hackedPayload, _ := json.Marshal(claims)
	tampered := parts[0] + "." + base64.RawURLEncoding.EncodeToString(hackedPayload) + "." + parts[2]

	_, err = issuer.ValidateToken(tampered)
	if err == nil {
		t.Error("expected error for tampered payload")
	}
}

func TestIssuer_ValidateToken_NBF_Future(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	issuer := NewIssuer(mgr)

	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      time.Hour,
		JWTID:    "jti-nbf",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	parts := strings.Split(token, ".")
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	claims["nbf"] = time.Now().Add(time.Hour).Unix()
	modifiedPayload, _ := json.Marshal(claims)

	signingInput := parts[0] + "." + base64.RawURLEncoding.EncodeToString(modifiedPayload)
	modifiedToken := signingInput + "." + parts[2]

	_, err = issuer.ValidateToken(modifiedToken)
	if err == nil {
		t.Error("expected error for future nbf")
	}
}

func TestIssuer_ValidateTokenWithConfig_IssuerMissing(t *testing.T) {
	issuer := newTestIssuer(t)

	payload := map[string]interface{}{
		"iss": "",
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"aud": []string{"app-1"},
	}

	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)
	token, err := adapter.SignToken(payload, "JWT")
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}

	_, err = issuer.ValidateTokenWithConfig(token, ValidationConfig{
		ExpectedIssuer: "https://auth.example.com",
	})
	if err == nil {
		t.Error("expected error for missing issuer")
	}
}

func TestIssuer_ValidateTokenWithConfig_AudienceArrayMismatch(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1", "app-2"},
		TTL:      time.Hour,
		JWTID:    "jti-aud-arr",
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
	if !ok || len(aud) != 2 {
		t.Errorf("aud = %v, want 2 elements", claims["aud"])
	}
}

func TestIssuer_ValidateToken_DecodeSignatureError(t *testing.T) {
	issuer := newTestIssuer(t)
	_, err := issuer.ValidateToken("a.b.!!!invalid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64 signature")
	}
}

func TestIssuer_ValidateToken_DecodePayloadError(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      time.Hour,
		JWTID:    "jti-dec",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	parts := strings.Split(token, ".")
	tampered := parts[0] + ".!!!invalid-base64!!!." + parts[2]
	_, err = issuer.ValidateToken(tampered)
	if err == nil {
		t.Error("expected error for invalid base64 payload")
	}
}

func TestIssuer_SignClaims_WithValidKey(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	issuer := NewIssuer(mgr)

	claims := AccessTokenClaims{
		Issuer:  "test",
		Subject: "sub1",
		Expiry:  time.Now().Add(time.Hour).Unix(),
	}
	token, err := issuer.signClaims(claims, "")
	if err != nil {
		t.Fatalf("signClaims() error = %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("expected 3 parts, got %d", len(parts))
	}
}

func TestIssuer_IssueAccessToken_ZeroTTL(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      0,
		JWTID:    "jti-zero",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	_, err = issuer.ValidateToken(token)
	if err == nil {
		t.Error("expected error for immediately expired token (TTL=0)")
	}
}

func TestIssuer_ValidateTokenWithConfig_EmptyIssuerConfig(t *testing.T) {
	issuer := newTestIssuer(t)
	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://auth.example.com",
		Subject:  "user-1",
		Audience: []string{"app-1"},
		TTL:      time.Hour,
		JWTID:    "jti-empty-iss",
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	_, err = issuer.ValidateTokenWithConfig(token, ValidationConfig{})
	if err != nil {
		t.Fatalf("ValidateTokenWithConfig() with empty config error = %v", err)
	}
}
