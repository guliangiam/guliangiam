package dpop

import "testing"

func TestAccessTokenHash_KNOWNVectors(t *testing.T) {
	if got := AccessTokenHash("access-token"); got != "Pxa-1wifRlPl7yG_0oJNfzqq7MelmOfonFgOFgapzFI" {
		t.Fatalf("AccessTokenHash() = %q", got)
	}
	if got := AccessTokenHash("Kz~8mXK1EalYznwH-LC-1fBAo.4Ljp~zsPE_NeO.gxU"); got != "fUHyO2r2Z3DZ53EsNrWBb0xWXoaNy59IiKCAqksmQEo" {
		t.Fatalf("AccessTokenHash() = %q", got)
	}
}

func TestValidateAccessTokenHash(t *testing.T) {
	token := "access-token"
	if err := ValidateAccessTokenHash(AccessTokenHash(token), token); err != nil {
		t.Fatalf("matching ath rejected: %v", err)
	}
	if err := ValidateAccessTokenHash(AccessTokenHash("other-token"), token); err == nil {
		t.Fatal("mismatched ath accepted")
	}
	if err := ValidateAccessTokenHash("", token); err == nil {
		t.Fatal("empty ath accepted")
	}
	if err := ValidateAccessTokenHash(AccessTokenHash(token), ""); err == nil {
		t.Fatal("empty access token accepted")
	}
}

func TestValidateResourceProof_MalformedProof(t *testing.T) {
	if _, err := ValidateResourceProof("not-a-jwt", "token", "GET", "https://api.example.com/users"); err == nil {
		t.Fatal("malformed proof accepted")
	}
	if _, err := ValidateResourceProof("", "token", "GET", "https://api.example.com/users"); err == nil {
		t.Fatal("empty proof accepted")
	}
}
