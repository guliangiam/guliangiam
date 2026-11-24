package auth

import (
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthorizeRequestFromQueryMapsStandardAndCamelCase(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/authorize?client_id=gl_cid_1&redirect_uri=https%3A%2F%2Fexample.com%2Fcb&response_type=code&scope=openid+profile&state=s1&nonce=n1&code_challenge=abc&code_challenge_method=S256&resource=urn%3Aapi", nil)
	got := authorizeRequestFromQuery(req)
	if got.ClientID != "gl_cid_1" || got.RedirectURI != "https://example.com/cb" || got.ResponseType != "code" {
		t.Fatalf("standard params mapped wrong: %+v", got)
	}
	if got.State != "s1" || got.Nonce != "n1" || got.CodeChallenge != "abc" || got.ChallengeMethod != "S256" {
		t.Fatalf("binding params mapped wrong: %+v", got)
	}
	if len(got.RequestedScopes) != 2 || got.RequestedScopes[0] != "openid" || got.RequestedScopes[1] != "profile" {
		t.Fatalf("scopes = %v", got.RequestedScopes)
	}
	if got.Resource != "urn:api" {
		t.Fatalf("resource = %q", got.Resource)
	}

	camel := httptest.NewRequest("GET", "/api/v1/auth/authorize?clientId=gl_cid_2&redirectUri=https%3A%2F%2Fexample.com%2Fcb&responseType=code&requestedScopes=openid", nil)
	gotCamel := authorizeRequestFromQuery(camel)
	if gotCamel.ClientID != "gl_cid_2" || gotCamel.ResponseType != "code" || len(gotCamel.RequestedScopes) != 1 {
		t.Fatalf("camelCase params mapped wrong: %+v", gotCamel)
	}
}

func TestAuthorizeConsentRedirectPreservesOriginalQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/authorize?client_id=gl_cid_1&state=s1", nil)
	got := authorizeConsentRedirect("app-demo", req)
	if !strings.HasPrefix(got, "/consent/app-demo?") {
		t.Fatalf("consent redirect = %q", got)
	}
	if !strings.Contains(got, "client_id=gl_cid_1") || !strings.Contains(got, "state=s1") {
		t.Fatalf("consent redirect lost params: %q", got)
	}
}

func TestAuthorizeRequestFromQueryIgnoresEmptyValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/authorize?client_id=&state=", nil)
	got := authorizeRequestFromQuery(req)
	if got.ClientID != "" || got.State != "" || got.RequestedScopes != nil {
		t.Fatalf("empty params must stay empty: %+v", got)
	}
	_ = base64.RawURLEncoding
}
