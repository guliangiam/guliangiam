package jwt

import (
	"context"
	"testing"

	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

func TestJWKSProviderAdapter_SignToken_WithClaims(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	claims := map[string]any{
		"sub": "user-sign",
		"iss": "https://auth.example.com",
		"aud": []string{"app-1"},
		"exp": 1735689600,
		"iat": 1735686000,
	}

	token, err := adapter.SignToken(claims, "JWT")
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}

	parts := splitToken(token)
	if len(parts) != 3 {
		t.Errorf("expected 3 JWT parts, got %d", len(parts))
	}
}

func TestJWKSProviderAdapter_SignToken_DifferentTypes(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	claims := map[string]any{"sub": "user-1"}

	jwtToken, err := adapter.SignToken(claims, "JWT")
	if err != nil {
		t.Fatalf("SignToken(JWT) error = %v", err)
	}

	atToken, err := adapter.SignToken(claims, "AT")
	if err != nil {
		t.Fatalf("SignToken(AT) error = %v", err)
	}

	if jwtToken == atToken {
		t.Error("different token types should produce different tokens")
	}
}

func TestJWKSProviderAdapter_GetKeyPair_Consistency(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	kp1, err := adapter.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}
	kp2, err := adapter.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}

	if kp1.Kid != kp2.Kid {
		t.Errorf("Kid mismatch: %s vs %s", kp1.Kid, kp2.Kid)
	}
	if kp1.Algorithm != kp2.Algorithm {
		t.Errorf("Algorithm mismatch: %s vs %s", kp1.Algorithm, kp2.Algorithm)
	}
}

func TestJWKSProviderAdapter_GetJWKS_KeyFields(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	jwks, err := adapter.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	if len(jwks.Keys) == 0 {
		t.Fatal("expected at least one key")
	}

	key := jwks.Keys[0]
	if key.Kid == "" {
		t.Error("expected non-empty Kid in JWK")
	}
}

func TestJWKSProviderAdapter_ImplementsDomainInterface(t *testing.T) {
	var provider domainauth.JWKSProvider = NewJWKSProviderAdapter(platformjwks.NewManager(2048, nil))
	_ = provider
}

func TestJWKSProviderAdapter_SignToken_NilClaims(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	token, err := adapter.SignToken(nil, "JWT")
	if err != nil {
		t.Fatalf("SignToken(nil) error = %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token even with nil claims")
	}
}

func TestJWKSProviderAdapter_GetPublicKey_KeyBits(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	pub, err := adapter.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey() error = %v", err)
	}
	if pub.N.BitLen() != 2048 {
		t.Errorf("expected 2048-bit key, got %d", pub.N.BitLen())
	}
}

func TestJWKSProviderAdapter_ContextMethods(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	ctx := context.Background()
	_ = ctx

	kp, err := adapter.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}
	if kp == nil {
		t.Error("expected non-nil key pair")
	}
}
