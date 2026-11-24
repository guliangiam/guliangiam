package redact

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeText_RedactsURLUserinfoAndQuery(t *testing.T) {
	const secret = "super-secret-key"
	input := "POST https://user:pass@mcp.example.com/tools?key=" + secret + "&api_key=sk-live-abc123 failed"

	got := SanitizeText(input)

	if strings.Contains(got, secret) {
		t.Fatalf("query value survived: %q", got)
	}
	for _, leaked := range []string{"user:pass", "?key=", "api_key=", "sk-live-abc123"} {
		if strings.Contains(got, leaked) {
			t.Errorf("fragment %q survived: %q", leaked, got)
		}
	}
	if !strings.Contains(got, "https://mcp.example.com") {
		t.Errorf("scheme+host should be preserved, got %q", got)
	}
}

func TestSanitizeText_RedactsCredentialHeaders(t *testing.T) {
	const apiKey = "sk-live-0123456789abcdef0123456789abcdef"
	tests := []struct {
		name  string
		input string
	}{
		{"colon", "upstream rejected x-api-key: " + apiKey},
		{"json body", `upstream body {"x-api-key":"` + apiKey + `"}`},
		{"authorization bearer", "Authorization: Bearer " + apiKey},
		{"authorization basic", "Authorization: Basic dXNlcjpwYXNz"},
		{"equals form", "api_key=" + apiKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeText(tt.input)
			if strings.Contains(got, apiKey) {
				t.Fatalf("header value survived: %q", got)
			}
			if !strings.Contains(got, Placeholder) {
				t.Errorf("expected placeholder in %q", got)
			}
		})
	}
}

func TestSanitizeText_RedactsBareLongTokens(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef" // 32 chars, hex-like
	got := SanitizeText("token dump " + token)
	if strings.Contains(got, token) {
		t.Fatalf("long token survived: %q", got)
	}
	if !strings.Contains(got, Placeholder) {
		t.Errorf("expected placeholder in %q", got)
	}
}

func TestSanitizeText_LeavesOrdinaryTextReadable(t *testing.T) {
	input := "mcp call tool srv/tool: upstream returned 500 Internal Server Error"
	if got := SanitizeText(input); got != input {
		t.Fatalf("ordinary text changed: got %q, want %q", got, input)
	}
}

func TestSanitizeText_ReplacesUnparsableURL(t *testing.T) {
	got := SanitizeText("call https://example.com/%zz failed")
	if strings.Contains(got, "%zz") {
		t.Fatalf("malformed URL survived: %q", got)
	}
	if !strings.Contains(got, Placeholder) {
		t.Errorf("expected placeholder in %q", got)
	}
}

func TestSanitizeText_IsBounded(t *testing.T) {
	got := SanitizeText(strings.Repeat("a", 300))
	if n := utf8.RuneCountInString(got); n > MaxTextRunes {
		t.Fatalf("sanitized text is %d runes, want <= %d", n, MaxTextRunes)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated text should end with an ellipsis: %q", got)
	}
}

func TestWrapError_PreservesChainAndRedacts(t *testing.T) {
	const secret = "super-secret-key"
	cause := fmt.Errorf("dial https://mcp.example.com/tools?key=%s: %w", secret, context.DeadlineExceeded)

	wrapped := WrapError(cause)

	if !errors.Is(wrapped, context.DeadlineExceeded) {
		t.Fatal("WrapError dropped the context.DeadlineExceeded chain")
	}
	if got := wrapped.Error(); strings.Contains(got, secret) {
		t.Fatalf("wrapped message leaked the query value: %q", got)
	}
	if !strings.Contains(wrapped.Error(), "https://mcp.example.com") {
		t.Errorf("wrapped message should keep scheme+host, got %q", wrapped.Error())
	}
}

func TestWrapError_Nil(t *testing.T) {
	if WrapError(nil) != nil {
		t.Fatal("WrapError(nil) should be nil")
	}
}
