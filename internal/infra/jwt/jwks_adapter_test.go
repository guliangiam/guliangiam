package jwt

import (
	"strings"
	"testing"

	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

func TestJWKSProviderAdapter_GetKeyPair(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	kp, err := adapter.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}
	if kp.Kid == "" {
		t.Error("expected non-empty Kid")
	}
	if kp.Algorithm != "RS256" {
		t.Errorf("Algorithm = %q, want RS256", kp.Algorithm)
	}
	if kp.Key == nil {
		t.Error("expected non-nil Key")
	}
}

func TestJWKSProviderAdapter_GetJWKS(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	jwks, err := adapter.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	if len(jwks.Keys) != 1 {
		t.Fatalf("len(Keys) = %d, want 1 (RSA only; SM2 is published by GetGMJWKS)", len(jwks.Keys))
	}
	rsaJWK := &jwks.Keys[0]
	if !strings.HasPrefix(rsaJWK.Kid, "rsa_") {
		t.Fatalf("expected an rsa_ key, got %+v", jwks.Keys)
	}
	if rsaJWK.Kty != "RSA" || rsaJWK.Alg != "RS256" || rsaJWK.N == "" || rsaJWK.E == "" {
		t.Errorf("RSA JWK = %+v", rsaJWK)
	}
	if rsaJWK.Crv != "" || rsaJWK.X != "" || rsaJWK.Y != "" {
		t.Errorf("standard JWKS leaked EC/SM2 parameters: %+v", rsaJWK)
	}
}

func TestJWKSProviderAdapter_GetGMJWKS(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	jwks, err := adapter.GetGMJWKS()
	if err != nil {
		t.Fatalf("GetGMJWKS() error = %v", err)
	}
	if len(jwks.Keys) != 1 {
		t.Fatalf("len(Keys) = %d, want 1 (SM2 only)", len(jwks.Keys))
	}
	gmJWK := &jwks.Keys[0]
	if !strings.HasPrefix(gmJWK.Kid, "gm_") {
		t.Fatalf("expected a gm_ key, got %+v", jwks.Keys)
	}
	if gmJWK.Kty != "EC" || gmJWK.Crv != "SM2" || gmJWK.Alg != "SM2-SM3" || gmJWK.X == "" || gmJWK.Y == "" {
		t.Errorf("SM2 JWK = %+v", gmJWK)
	}
	if gmJWK.N != "" || gmJWK.E != "" {
		t.Errorf("GM JWKS leaked RSA parameters: %+v", gmJWK)
	}
}

func TestJWKSProviderAdapter_GetPublicKey(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	pub, err := adapter.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey() error = %v", err)
	}
	if pub == nil {
		t.Error("expected non-nil public key")
	}
	if pub.N.BitLen() == 0 {
		t.Error("expected non-zero bit length")
	}
}

func TestJWKSProviderAdapter_SignToken(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	claims := map[string]any{
		"sub": "user-1",
		"iss": "https://auth.example.com",
		"aud": "app-1",
	}

	token, err := adapter.SignToken(claims, "JWT")
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestNewJWKSProviderAdapter(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)
	if adapter == nil {
		t.Error("expected non-nil adapter")
	}
}

func TestJWKSProviderAdapter_ImplementsInterface(t *testing.T) {
	var _ domainauth.JWKSProvider = (*JWKSProviderAdapter)(nil)
}
