package jwt

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

func sm2IssuerParams() IssueIDTokenParams {
	return IssueIDTokenParams{
		Issuer:             "https://auth.example.com",
		Algorithm:          platformjwks.SigningAlgorithmSM2SM3,
		Subject:            "user-1",
		Audience:           []string{"app-1"},
		AuthenticationTime: time.Now().UTC(),
	}
}

func TestIssuer_IssueIDToken_SM2SM3(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	issuer := NewIssuer(mgr)

	token, err := issuer.IssueIDToken(sm2IssuerParams())
	if err != nil {
		t.Fatalf("IssueIDToken(SM2-SM3): %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token parts = %d", len(parts))
	}
	rawHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
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

	claims, err := issuer.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken(SM2-SM3): %v", err)
	}
	if claims["sub"] != "user-1" {
		t.Fatalf("sub = %v", claims["sub"])
	}
}

func TestIssuer_RS256DefaultStillWorks(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	issuer := NewIssuer(mgr)
	params := sm2IssuerParams()
	params.Algorithm = ""

	token, err := issuer.IssueIDToken(params)
	if err != nil {
		t.Fatalf("IssueIDToken(default): %v", err)
	}
	if _, err := issuer.ValidateToken(token); err != nil {
		t.Fatalf("ValidateToken(RS256): %v", err)
	}

	rawHeader, _ := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[0])
	var header map[string]string
	_ = json.Unmarshal(rawHeader, &header)
	if header["alg"] != platformjwks.SigningAlgorithmRS256 {
		t.Fatalf("default alg = %q", header["alg"])
	}
	if !strings.HasPrefix(header["kid"], "rsa_") {
		t.Fatalf("default kid = %q, want rsa_ prefix", header["kid"])
	}
}

func TestIssuer_CrossAlgorithmKidRejected(t *testing.T) {
	mgr := platformjwks.NewManager(2048, nil)
	issuer := NewIssuer(mgr)

	sm2Token, err := issuer.IssueIDToken(sm2IssuerParams())
	if err != nil {
		t.Fatalf("IssueIDToken: %v", err)
	}
	parts := strings.Split(sm2Token, ".")
	rawHeader, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var header map[string]string
	_ = json.Unmarshal(rawHeader, &header)
	header["alg"] = platformjwks.SigningAlgorithmRS256
	mutatedHeader, _ := json.Marshal(header)
	mutated := base64.RawURLEncoding.EncodeToString(mutatedHeader) + "." + parts[1] + "." + parts[2]

	if _, err := issuer.ValidateToken(mutated); err == nil {
		t.Fatal("RS256 with a gm_ kid must be rejected")
	}
}
