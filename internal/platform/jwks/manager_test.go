package jwks

import (
	"strings"

	"crypto/rand"
	"crypto/rsa"
	"testing"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager(2048, nil)
	if mgr == nil {
		t.Error("expected non-nil manager")
	}
	if mgr.keySize != 2048 {
		t.Errorf("keySize = %d, want 2048", mgr.keySize)
	}
}

func TestManager_GetKeyPair(t *testing.T) {
	mgr := NewManager(2048, nil)
	kp, err := mgr.GetKeyPair()
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
		t.Error("expected non-nil private key")
	}
}

func TestManager_GetKeyPair_Cached(t *testing.T) {
	mgr := NewManager(2048, nil)
	kp1, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("first GetKeyPair() error = %v", err)
	}
	kp2, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("second GetKeyPair() error = %v", err)
	}
	if kp1.Kid != kp2.Kid {
		t.Errorf("Kid changed: %q != %q", kp1.Kid, kp2.Kid)
	}
}

func TestManager_GetKeyPair_CustomKidFn(t *testing.T) {
	customKid := "my-custom-kid"
	mgr := NewManager(2048, func() (string, error) {
		return customKid, nil
	})
	kp, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}
	if kp.Kid != KidPrefixRSA+customKid {
		t.Errorf("Kid = %q, want %q", kp.Kid, KidPrefixRSA+customKid)
	}
}

func TestManager_GetKeyPair_4096KeySize(t *testing.T) {
	mgr := NewManager(4096, nil)
	kp, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}
	if kp.Key.N.BitLen() < 4096 {
		t.Errorf("key bit length = %d, want >= 4096", kp.Key.N.BitLen())
	}
}

func TestManager_GetJWKS(t *testing.T) {
	mgr := NewManager(2048, nil)
	jwks, err := mgr.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	if len(jwks.Keys) != 1 {
		t.Fatalf("len(Keys) = %d, want 1 (RSA only; SM2 is published by GetGMJWKS)", len(jwks.Keys))
	}
	rsaJWK := &jwks.Keys[0]
	if !strings.HasPrefix(rsaJWK.Kid, KidPrefixRSA) {
		t.Fatalf("expected an rsa_ key, got %+v", jwks.Keys)
	}
	if rsaJWK.Kty != "RSA" || rsaJWK.Alg != SigningAlgorithmRS256 || rsaJWK.N == "" || rsaJWK.E == "" {
		t.Errorf("RSA JWK = %+v", rsaJWK)
	}
	if rsaJWK.Crv != "" || rsaJWK.X != "" || rsaJWK.Y != "" {
		t.Errorf("RSA JWK carries EC parameters: %+v", rsaJWK)
	}
}

func TestManager_GetGMJWKS(t *testing.T) {
	mgr := NewManager(2048, nil)
	jwks, err := mgr.GetGMJWKS()
	if err != nil {
		t.Fatalf("GetGMJWKS() error = %v", err)
	}
	if len(jwks.Keys) != 1 {
		t.Fatalf("len(Keys) = %d, want 1 (SM2 only)", len(jwks.Keys))
	}
	gmJWK := &jwks.Keys[0]
	if !strings.HasPrefix(gmJWK.Kid, KidPrefixGM) {
		t.Fatalf("expected a gm_ key, got %+v", jwks.Keys)
	}
	if gmJWK.Kty != "EC" || gmJWK.Crv != "SM2" || gmJWK.Alg != SigningAlgorithmSM2SM3 || gmJWK.X == "" || gmJWK.Y == "" {
		t.Errorf("SM2 JWK = %+v", gmJWK)
	}
	if gmJWK.N != "" || gmJWK.E != "" {
		t.Errorf("SM2 JWK carries RSA parameters: %+v", gmJWK)
	}
}

func TestEncodeRSAPublicKeyDER(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	der, err := EncodeRSAPublicKeyDER(&key.PublicKey)
	if err != nil {
		t.Fatalf("EncodeRSAPublicKeyDER() error = %v", err)
	}
	if len(der) == 0 {
		t.Error("expected non-empty DER encoding")
	}
}

func TestRandomKid(t *testing.T) {
	kid1, err := randomKid()
	if err != nil {
		t.Fatalf("randomKid() error = %v", err)
	}
	if len(kid1) != 32 {
		t.Errorf("len(kid) = %d, want 32 (16 bytes hex)", len(kid1))
	}
	kid2, err := randomKid()
	if err != nil {
		t.Fatalf("randomKid() error = %v", err)
	}
	if kid1 == kid2 {
		t.Error("two random KIDs should differ")
	}
}

// TestManager_GetKeyPairByID pins the rule a verifier depends on: an unknown kid
// is an error, not a reason to fall back to the current key. Falling back would
// make the kid decorative and let a token minted for a rotated-out key keep
// verifying after a rotation (设计报告 14.8, ADR-015).
func TestManager_GetKeyPairByID(t *testing.T) {
	mgr := NewManager(2048, nil)
	current, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}

	t.Run("known kid", func(t *testing.T) {
		got, err := mgr.GetKeyPairByID(current.Kid)
		if err != nil {
			t.Fatalf("GetKeyPairByID(%q) error = %v", current.Kid, err)
		}
		if got.Key != current.Key {
			t.Fatal("returned a different key for a known kid")
		}
	})

	t.Run("unknown kid", func(t *testing.T) {
		if _, err := mgr.GetKeyPairByID("not-a-known-kid"); err == nil {
			t.Fatal("expected an unknown kid to be rejected")
		}
	})

	t.Run("empty kid", func(t *testing.T) {
		if _, err := mgr.GetKeyPairByID("   "); err == nil {
			t.Fatal("expected an empty kid to be rejected")
		}
	})
}
