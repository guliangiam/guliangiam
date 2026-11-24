package redact

import (
	"strings"
	"testing"
)

func TestSanitizeQuery_KeepsNamesMasksValues(t *testing.T) {
	got := SanitizeQuery("access_token=secret-value&api_key=abc&page=2")
	want := "access_token=***&api_key=***&page=***"
	if got != want {
		t.Fatalf("SanitizeQuery() = %q, want %q", got, want)
	}
}

func TestSanitizeQuery_Empty(t *testing.T) {
	if got := SanitizeQuery(""); got != "" {
		t.Fatalf("SanitizeQuery(\"\") = %q, want empty", got)
	}
}

func TestSanitizeQuery_UnparsableCollapsesToPlaceholder(t *testing.T) {
	for _, raw := range []string{"a=1;b=2", "%zz=1", "a=%"} {
		if got := SanitizeQuery(raw); got != Placeholder {
			t.Errorf("SanitizeQuery(%q) = %q, want %q", raw, got, Placeholder)
		}
	}
}

func TestSanitizeQuery_RejectsTokenShapedNames(t *testing.T) {
	longKey := strings.Repeat("A", 40)
	got := SanitizeQuery(longKey + "=1")
	if strings.Contains(got, longKey) {
		t.Fatalf("SanitizeQuery leaked token-shaped key: %q", got)
	}
	if got != Placeholder+"="+Placeholder {
		t.Fatalf("SanitizeQuery() = %q, want %q", got, Placeholder+"="+Placeholder)
	}
}

func TestSanitizeQuery_CapsParameterCount(t *testing.T) {
	parts := make([]string, 0, maxQueryParams+4)
	for i := 0; i < maxQueryParams+4; i++ {
		parts = append(parts, "p"+string(rune('a'+i))+"=v")
	}
	got := SanitizeQuery(strings.Join(parts, "&"))
	if count := strings.Count(got, "="); count > maxQueryParams {
		t.Fatalf("SanitizeQuery kept %d names, want <= %d: %q", count, maxQueryParams, got)
	}
}

func TestSanitizeQuery_IsBounded(t *testing.T) {
	query := strings.Repeat("some_long_parameter_name=value&", 20)
	got := SanitizeQuery(query)
	if len([]rune(got)) > MaxTextRunes {
		t.Fatalf("SanitizeQuery() length %d exceeds %d", len([]rune(got)), MaxTextRunes)
	}
}
