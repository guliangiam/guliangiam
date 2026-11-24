package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

// These tests pin 设计报告 14.8: an explicit JOSE algorithm allow-list,
// rejection of algorithm confusion, and key selection by the presented kid.
// Before this, the verifier always used the current key and never looked at the
// header, so an unknown kid was accepted and no rejection was ever named.

func signWithKey(t *testing.T, key *rsa.PrivateKey, kid, alg string, claims map[string]any, hash crypto.Hash) string {
	t.Helper()

	header := map[string]any{"alg": alg, "typ": "JWT"}
	if kid != "" {
		header["kid"] = kid
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)

	var digest []byte
	if hash != 0 {
		hasher := hash.New()
		hasher.Write([]byte(signingInput))
		digest = hasher.Sum(nil)
	}

	signature := []byte("unsigned-signature")
	if key != nil {
		signed, err := rsa.SignPKCS1v15(rand.Reader, key, hash, digest)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		signature = signed
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func issuerWithKey(t *testing.T) (*Issuer, *platformjwks.Manager) {
	t.Helper()
	mgr := platformjwks.NewManager(2048, nil)
	if _, err := mgr.GetKeyPair(); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return NewIssuer(mgr), mgr
}

func validClaims() map[string]any {
	now := time.Now().UTC()
	return map[string]any{
		"iss": "https://iam.example.com",
		"sub": "user-1",
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
}

func TestValidateToken_RejectsAlgNone(t *testing.T) {
	issuer, mgr := issuerWithKey(t)
	kp, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("get key pair: %v", err)
	}

	// `alg: none` with a non-empty signature: rejecting this must come from the
	// allow-list, not from the signature check happening to fail.
	token := signWithKey(t, nil, kp.Kid, "none", validClaims(), 0)

	if _, err := issuer.ValidateToken(token); err == nil {
		t.Fatal("expected alg=none to be rejected")
	}
}

func TestValidateToken_RejectsAsymmetricAlgorithmsOtherThanRS256(t *testing.T) {
	issuer, mgr := issuerWithKey(t)
	kp, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("get key pair: %v", err)
	}

	for _, alg := range []string{"HS256", "RS384", "PS256", "ES256", "none"} {
		t.Run(alg, func(t *testing.T) {
			token := signWithKey(t, kp.Key, kp.Kid, alg, validClaims(), crypto.SHA256)
			if _, err := issuer.ValidateToken(token); err == nil {
				t.Fatalf("expected alg=%s to be rejected", alg)
			}
		})
	}
}

func TestValidateToken_RejectsMissingKid(t *testing.T) {
	issuer, mgr := issuerWithKey(t)
	kp, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("get key pair: %v", err)
	}

	token := signWithKey(t, kp.Key, "", "RS256", validClaims(), crypto.SHA256)

	if _, err := issuer.ValidateToken(token); err == nil {
		t.Fatal("expected a token without kid to be rejected")
	}
}

func TestValidateToken_RejectsUnknownKid(t *testing.T) {
	issuer, mgr := issuerWithKey(t)
	kp, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("get key pair: %v", err)
	}

	// Signed with the right key but presenting another kid: accepting this would
	// mean the kid is decorative, so a token minted for a rotated-out key would
	// still verify after the rotation completes.
	token := signWithKey(t, kp.Key, "some-other-kid", "RS256", validClaims(), crypto.SHA256)

	if _, err := issuer.ValidateToken(token); err == nil {
		t.Fatal("expected an unknown kid to be rejected")
	}
}

func TestValidateToken_RejectsMalformedHeader(t *testing.T) {
	issuer, _ := issuerWithKey(t)

	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user-1"}`))
	cases := map[string]string{
		"not base64":     "!!!." + payload + ".sig",
		"not json":       base64.RawURLEncoding.EncodeToString([]byte("not-json")) + "." + payload + ".sig",
		"missing alg":    base64.RawURLEncoding.EncodeToString([]byte(`{"kid":"k"}`)) + "." + payload + ".sig",
		"empty payload":  base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`)) + "." + payload + ".sig",
		"too many parts": "a.b.c.d",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := issuer.ValidateToken(token); err == nil {
				t.Fatalf("expected %s to be rejected", name)
			}
		})
	}
}

func TestValidateToken_RejectsCriticalHeaderParameters(t *testing.T) {
	issuer, mgr := issuerWithKey(t)
	kp, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("get key pair: %v", err)
	}

	// RFC 7515 §4.1.11: a verifier that does not understand every `crit` entry
	// must reject the token. This verifier understands none of them.
	headerJSON, err := json.Marshal(map[string]any{
		"alg":  "RS256",
		"typ":  "JWT",
		"kid":  kp.Kid,
		"crit": []string{"exp"},
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadJSON, err := json.Marshal(validClaims())
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, kp.Key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	token := signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)

	if _, err := issuer.ValidateToken(token); err == nil {
		t.Fatal("expected a token with critical header parameters to be rejected")
	}
}

func TestValidateToken_AcceptsItsOwnToken(t *testing.T) {
	issuer, _ := issuerWithKey(t)

	token, err := issuer.IssueAccessToken(IssueAccessTokenParams{
		Issuer:   "https://iam.example.com",
		Subject:  "user-1",
		Audience: []string{"portal-client"},
		TTL:      time.Hour,
	})
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	claims, err := issuer.ValidateTokenWithConfig(token, ValidationConfig{ExpectedIssuer: "https://iam.example.com"})
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims["sub"] != "user-1" {
		t.Fatalf("sub = %v, want user-1", claims["sub"])
	}
}

func TestValidateToken_StillChecksExpiryAndIssuer(t *testing.T) {
	issuer, mgr := issuerWithKey(t)
	kp, err := mgr.GetKeyPair()
	if err != nil {
		t.Fatalf("get key pair: %v", err)
	}

	expired := validClaims()
	expired["exp"] = time.Now().Add(-time.Minute).Unix()
	if _, err := issuer.ValidateToken(signWithKey(t, kp.Key, kp.Kid, "RS256", expired, crypto.SHA256)); err == nil {
		t.Fatal("expected an expired token to be rejected")
	}

	wrongIssuer := validClaims()
	wrongIssuer["iss"] = "https://attacker.example.com"
	token := signWithKey(t, kp.Key, kp.Kid, "RS256", wrongIssuer, crypto.SHA256)
	if _, err := issuer.ValidateTokenWithConfig(token, ValidationConfig{ExpectedIssuer: "https://iam.example.com"}); err == nil {
		t.Fatal("expected a mismatched issuer to be rejected")
	}
}
