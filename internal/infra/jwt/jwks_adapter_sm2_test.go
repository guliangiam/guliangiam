package jwt

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

func TestJWKSProviderAdapter_SignTokenWithSM2SM3(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	adapter := NewJWKSProviderAdapter(mgr)

	token, err := adapter.SignTokenWithAlgorithm(map[string]any{"sub": "user-1"}, "JWT", platformjwks.SigningAlgorithmSM2SM3)
	if err != nil {
		t.Fatalf("SignTokenWithAlgorithm(SM2-SM3): %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token parts = %d", len(parts))
	}
	rawHeader, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var header map[string]string
	if err := json.Unmarshal(rawHeader, &header); err != nil {
		t.Fatalf("parse header: %v", err)
	}
	if header["alg"] != platformjwks.SigningAlgorithmSM2SM3 {
		t.Fatalf("alg = %q", header["alg"])
	}
	if !strings.HasPrefix(header["kid"], "gm_") {
		t.Fatalf("kid = %q, want gm_ prefix", header["kid"])
	}

	gm, err := mgr.GetGMKeyPairByID(header["kid"])
	if err != nil {
		t.Fatalf("GetGMKeyPairByID: %v", err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	digest := platformcrypto.SM3Sum([]byte(parts[0] + "." + parts[1]))
	if !platformcrypto.NewCryptoSuite().SM2Verify(gm.PublicKey, digest, signature) {
		t.Fatal("JWKS gm_ public key did not verify the SM2-SM3 signature")
	}
}
