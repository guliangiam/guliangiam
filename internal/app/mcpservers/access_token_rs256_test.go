package mcpservers

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	infrajwt "github.com/guliangiam/guliangiam/internal/infra/jwt"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

// decodeMCPJWTHeader parses the JOSE header of an MCP access token. Asserting on
// the signed token (not on a stored parameter) keeps a later change inside the
// adapter visible to this test.
func decodeMCPJWTHeader(t *testing.T, token string) map[string]string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("MCP JWT parts = %d, want 3", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode MCP JWT JOSE header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(raw, &header); err != nil {
		t.Fatalf("parse MCP JWT JOSE header: %v", err)
	}
	return header
}

// TestSignMCPJWT_PinsRS256AndRSAKid pins the MCP access token carrier contract
// with the real JWKS adapter. The token is a system-level JWT: signMCPJWT calls
// SignToken, which has no token_signing_method input, so the adapter signs RS256
// with an rsa_ kid. Application-level algorithm selection does not extend to MCP
// access tokens; supporting SM2-SM3 here would need a separate decision and an
// explicit algorithm argument on that call path. A failure here means the MCP
// verification key family changed silently.
func TestSignMCPJWT_PinsRS256AndRSAKid(t *testing.T) {
	provider := infrajwt.NewJWKSProviderAdapter(platformjwks.NewManager(2048, nil))
	claims := map[string]any{
		"iss":       "guliangiam",
		"sub":       "user-1",
		"aud":       "mcp-server:server-1",
		"iat":       int64(1760000000),
		"exp":       int64(1760000300),
		"scope":     "mcp:tools",
		"server_id": "server-1",
	}

	token, err := signMCPJWT(claims, provider)
	if err != nil {
		t.Fatalf("signMCPJWT() error = %v", err)
	}

	header := decodeMCPJWTHeader(t, token)
	if got := header["alg"]; got != platformjwks.SigningAlgorithmRS256 {
		t.Fatalf("MCP JWT header alg = %q, want %q", got, platformjwks.SigningAlgorithmRS256)
	}
	if kid := header["kid"]; !strings.HasPrefix(kid, platformjwks.KidPrefixRSA) {
		t.Fatalf("MCP JWT header kid = %q, want %q prefix", kid, platformjwks.KidPrefixRSA)
	}
}
