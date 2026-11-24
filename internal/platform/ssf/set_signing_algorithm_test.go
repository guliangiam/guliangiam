package ssf

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	infrajwt "github.com/guliangiam/guliangiam/internal/infra/jwt"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

// decodeSETHeader parses the JOSE header of a signed SET. The test asserts on
// the bytes the real signer produced, so a header rewrite inside the adapter
// cannot be masked by a helper.
func decodeSETHeader(t *testing.T, token string) map[string]string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("SET parts = %d, want 3", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode SET JOSE header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(raw, &header); err != nil {
		t.Fatalf("parse SET JOSE header: %v", err)
	}
	return header
}

// TestSignEvent_PinsRS256AndRSAKid pins the SET carrier contract with the real
// JWKS adapter rather than the fake signer used above. A SET is a system-level
// JWT: it has no application context and therefore no token_signing_method, and
// the ssf.Signer port has no algorithm parameter at all. RS256 with an rsa_ kid
// is the current behaviour; adding SM2-SM3 for SETs would require its own
// decision and a signer port that can carry an algorithm, not an implicit
// fallback. If this assertion ever fails, the security event token silently
// changed its verification key family.
func TestSignEvent_PinsRS256AndRSAKid(t *testing.T) {
	signer := infrajwt.NewJWKSProviderAdapter(platformjwks.NewManager(2048, nil))

	token, err := SignEvent(signer, Event{
		Type:     EventCredentialChange,
		Issuer:   "https://op.example.com",
		Audience: "https://receiver.example.com",
		EventID:  "evt-rs256",
		IssuedAt: time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SignEvent() error = %v", err)
	}

	header := decodeSETHeader(t, token)
	if got := header["alg"]; got != platformjwks.SigningAlgorithmRS256 {
		t.Fatalf("SET header alg = %q, want %q (SETs do not follow token_signing_method)",
			got, platformjwks.SigningAlgorithmRS256)
	}
	if kid := header["kid"]; !strings.HasPrefix(kid, platformjwks.KidPrefixRSA) {
		t.Fatalf("SET header kid = %q, want %q prefix", kid, platformjwks.KidPrefixRSA)
	}
	if got := header["typ"]; got != "secevent+jwt" {
		t.Fatalf("SET header typ = %q, want secevent+jwt", got)
	}
}
