package keys

import (
	"crypto/rsa"
	"encoding/json"
	"testing"

	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
)

type fakeJWKSProvider struct {
	set *domainauth.JWKS
}

func (f fakeJWKSProvider) GetJWKS() (*domainauth.JWKS, error) {
	return f.set, nil
}

func (f fakeJWKSProvider) GetGMJWKS() (*domainauth.JWKS, error) {
	return f.set, nil
}

func (f fakeJWKSProvider) GetKeyPair() (*domainauth.KeyPair, error) { return nil, nil }
func (f fakeJWKSProvider) GetPublicKey() (*rsa.PublicKey, error)    { return nil, nil }
func (f fakeJWKSProvider) SignToken(map[string]any, string) (string, error) {
	return "", nil
}
func (f fakeJWKSProvider) SignTokenWithAlgorithm(map[string]any, string, string) (string, error) {
	return "", nil
}

func TestJWKSViewProvider_PreservesSM2Coordinates(t *testing.T) {
	view := NewJWKSViewProvider(fakeJWKSProvider{set: &domainauth.JWKS{Keys: []domainauth.JWK{
		{Kty: "RSA", Use: "sig", Kid: "rsa_test", Alg: "RS256", N: "n-value", E: "AQAB"},
		{Kty: "EC", Use: "sig", Kid: "gm_test", Alg: "SM2-SM3", Crv: "SM2", X: "x-value", Y: "y-value"},
	}}})

	set, err := view.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS: %v", err)
	}
	if len(set.Keys) != 2 {
		t.Fatalf("keys = %d, want 2", len(set.Keys))
	}

	var rsaKey, gmKey *JWK
	for i := range set.Keys {
		switch set.Keys[i].Kid {
		case "rsa_test":
			rsaKey = &set.Keys[i]
		case "gm_test":
			gmKey = &set.Keys[i]
		}
	}
	if rsaKey == nil || rsaKey.N == "" || rsaKey.E == "" || rsaKey.Crv != "" {
		t.Fatalf("RSA key = %+v", rsaKey)
	}
	if gmKey == nil || gmKey.Crv != "SM2" || gmKey.X == "" || gmKey.Y == "" || gmKey.Alg != "SM2-SM3" {
		t.Fatalf("GM key = %+v", gmKey)
	}

	// The transport serializes JWKS directly, so assert the wire shape too:
	// RSA keeps n/e, SM2 carries crv/x/y and omits n/e.
	raw, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("marshal JWKS: %v", err)
	}
	var wire struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal JWKS: %v", err)
	}
	for _, key := range wire.Keys {
		switch key["kid"] {
		case "rsa_test":
			if key["n"] == nil || key["e"] == nil {
				t.Fatalf("RSA wire key missing n/e: %v", key)
			}
			if _, ok := key["crv"]; ok {
				t.Fatalf("RSA wire key unexpectedly has crv: %v", key)
			}
		case "gm_test":
			if key["crv"] != "SM2" || key["x"] == nil || key["y"] == nil || key["alg"] != "SM2-SM3" {
				t.Fatalf("SM2 wire key missing coordinates: %v", key)
			}
			if _, ok := key["n"]; ok {
				t.Fatalf("SM2 wire key unexpectedly has n: %v", key)
			}
		}
	}
}
