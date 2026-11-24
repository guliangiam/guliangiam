package kerberos

import (
	"testing"
)

func TestParseAuthorizationHeader(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{"empty", "", "", false},
		{"basic auth", "Basic abc123", "", false},
		{"negotiate with token", "Negotiate YIIBhwYBAoKCASQ=", "YIIBhwYBAoKCASQ=", true},
		{"negotiate lowercase", "negotiate YIIBhwYBAoKCASQ=", "YIIBhwYBAoKCASQ=", true},
		{"negotiate mixed case", "NeGoTiAtE abc", "abc", true},
		{"negotiate no space", "Negotiate", "", false},
		{"negotiate empty token", "Negotiate ", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := ParseAuthorizationHeader(tt.header)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if token != tt.wantToken {
				t.Errorf("token = %q, want %q", token, tt.wantToken)
			}
		})
	}
}

func TestBuildNegotiateResponse(t *testing.T) {
	if v := BuildNegotiateResponse(""); v != "Negotiate" {
		t.Errorf("empty token: got %q, want %q", v, "Negotiate")
	}
	if v := BuildNegotiateResponse("abc"); v != "Negotiate abc" {
		t.Errorf("with token: got %q, want %q", v, "Negotiate abc")
	}
}

func TestNewAuthenticator(t *testing.T) {
	a := NewAuthenticator("EXAMPLE.COM", "/nonexistent/keytab", "HTTP")
	if a.IsReady() {
		t.Error("expected IsReady=false with nonexistent keytab")
	}
}

func TestValidatedPrincipal_Fields(t *testing.T) {
	p := &ValidatedPrincipal{
		Username:    "alice",
		Realm:       "EXAMPLE.COM",
		DisplayName: "alice@EXAMPLE.COM",
	}
	if p.Username != "alice" {
		t.Errorf("expected alice, got %s", p.Username)
	}
	if p.Realm != "EXAMPLE.COM" {
		t.Errorf("expected EXAMPLE.COM, got %s", p.Realm)
	}
	if p.DisplayName != "alice@EXAMPLE.COM" {
		t.Errorf("expected alice@EXAMPLE.COM, got %s", p.DisplayName)
	}
}
