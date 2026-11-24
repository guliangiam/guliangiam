package webauthn

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RP ID 推导：结果必须是每个来源主机的后缀（WebAuthn 的硬约束），否则浏览器会拒绝 ceremony。

func TestRelyingPartyIDFor(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
		want    string
	}{
		{"single origin keeps its host", []string{"https://iam.example.com"}, "iam.example.com"},
		{"port and path are ignored", []string{"http://localhost:7101/login"}, "localhost"},
		{"sibling subdomains fall back to the parent domain", []string{"https://console.example.com", "https://iam.example.com"}, "example.com"},
		{"mix of parent and child keeps the parent", []string{"https://example.com", "https://iam.example.com"}, "example.com"},
		{"duplicates collapse", []string{"https://iam.example.com", "https://iam.example.com"}, "iam.example.com"},
		{"wildcard entries are skipped, the rest still decides", []string{"https://*.example.com", "https://iam.example.com"}, "iam.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RelyingPartyIDFor(tt.origins)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRelyingPartyIDFor_RefusesUnusableScopes(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
	}{
		{"empty allowlist", nil},
		{"only unusable entries", []string{"https://*.example.com", "not a url"}},
		{"different registrable domains", []string{"https://iam.example.com", "https://iam.example.org"}},
		{"single label that is not localhost", []string{"https://acme"}},
		{"tenant slug is not a domain", []string{"acme"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RelyingPartyIDFor(tt.origins)
			require.Error(t, err, "不可用的作用域必须报错，而不是交给浏览器拒绝")
			assert.Empty(t, got)
		})
	}
}

func TestCommonLabelSuffix_IsLabelAligned(t *testing.T) {
	// 按标签对齐（而不是字符串后缀）：notexample.com 与 example.com 只共享顶层标签 "com"，
	// 因此不会把前者误当成后者的子域。只剩顶层标签的结果由 RelyingPartyIDFor 拒绝
	// （"com" 不是可用的 RP ID），这里只断言对齐本身。
	assert.Equal(t, "com", commonLabelSuffix("notexample.com", "example.com"))
	assert.Equal(t, "example.com", commonLabelSuffix("iam.example.com", "console.example.com"))
	assert.Equal(t, "iam.example.com", commonLabelSuffix("iam.example.com", "iam.example.com"))
	assert.Equal(t, "", commonLabelSuffix("example.com", "example.org"))
}
