package dcr

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// The OpenAPI contract declares jwks on both DCR request bodies. route.DecodeJSON
// calls DisallowUnknownFields, so a DTO that drops the field turns a valid
// private_key_jwt registration into a generic 400 before the service can accept
// or reject it. These tests pin the transport half of that contract.

func TestRegisterClientRequest_DecodesJWKSAndJWKSURI(t *testing.T) {
	body := `{
		"client_name": "rp",
		"redirect_uris": ["https://rp.example/cb"],
		"token_endpoint_auth_method": "private_key_jwt",
		"jwks": {"keys": [{"kty": "RSA", "kid": "k1"}]},
		"jwks_uri": "https://rp.example/jwks.json"
	}`
	request := httptest.NewRequest(http.MethodPost, "/oauth2/register", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	var decoded RegisterClientRequest
	if err := route.DecodeJSON(request, &decoded); err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if len(decoded.JWKS) == 0 || !strings.Contains(string(decoded.JWKS), `"kid"`) || !strings.Contains(string(decoded.JWKS), `"k1"`) {
		t.Fatalf("jwks = %s, want the embedded key set", decoded.JWKS)
	}
	if decoded.JWKSURI != "https://rp.example/jwks.json" {
		t.Fatalf("jwks_uri = %q", decoded.JWKSURI)
	}
}

func TestUpdateClientRequest_DecodesAuthMethodAndJWKS(t *testing.T) {
	body := `{
		"redirect_uris": ["https://rp.example/cb"],
		"token_endpoint_auth_method": "private_key_jwt",
		"jwks": {"keys": [{"kty": "RSA", "kid": "k2"}]}
	}`
	request := httptest.NewRequest(http.MethodPut, "/oauth2/register/client-abc", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	var decoded UpdateClientRequest
	if err := route.DecodeJSON(request, &decoded); err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if decoded.TokenEndpointAuthMethod != "private_key_jwt" {
		t.Fatalf("token_endpoint_auth_method = %q", decoded.TokenEndpointAuthMethod)
	}
	if len(decoded.JWKS) == 0 || !strings.Contains(string(decoded.JWKS), `"kid"`) || !strings.Contains(string(decoded.JWKS), `"k2"`) {
		t.Fatalf("jwks = %s, want the embedded key set", decoded.JWKS)
	}
}
