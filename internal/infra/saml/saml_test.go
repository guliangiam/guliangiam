package saml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no escaping", "hello", "hello"},
		{"ampersand", "a&b", "a&amp;b"},
		{"less than", "a<b", "a&lt;b"},
		{"greater than", "a>b", "a&gt;b"},
		{"double quote", `a"b`, "a&quot;b"},
		{"single quote", "a'b", "a&apos;b"},
		{"multiple", `<a>&"b'`, "&lt;a&gt;&amp;&quot;b&apos;"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeXML(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildUnsignedSAMLResponse(t *testing.T) {
	xml := buildUnsignedSAMLResponse(
		"resp-123", "assert-456",
		"2025-01-01T00:00:00Z", "2025-01-01T00:05:00Z",
		"req-789", "https://sp.example.com/acs",
		map[string]string{"email": "user@example.com"},
		"user@example.com", "https://idp.example.com",
	)
	assert.Contains(t, xml, "resp-123")
	assert.Contains(t, xml, "assert-456")
	assert.Contains(t, xml, "user@example.com")
	assert.Contains(t, xml, "https://idp.example.com")
	assert.Contains(t, xml, "https://sp.example.com/acs")
}

func TestBuildUnsignedSAMLResponse_NoAttributes(t *testing.T) {
	xml := buildUnsignedSAMLResponse(
		"resp-1", "assert-1",
		"2025-01-01T00:00:00Z", "2025-01-01T00:05:00Z",
		"req-1", "https://sp.example.com/acs",
		map[string]string{},
		"user@example.com", "https://idp.example.com",
	)
	assert.Contains(t, xml, "resp-1")
	assert.Contains(t, xml, "AttributeStatement")
}

func TestBuildUnsignedSAMLResponse_SpecialCharsInIssuer(t *testing.T) {
	xml := buildUnsignedSAMLResponse(
		"resp-1", "assert-1",
		"2025-01-01T00:00:00Z", "2025-01-01T00:05:00Z",
		"req-1", "https://sp.example.com/acs",
		nil,
		"user@example.com", "https://idp.example.com&a=test",
	)
	assert.Contains(t, xml, "&amp;")
}

func TestBuildSAMLMetadataWithKey_NilCert(t *testing.T) {
	metadata := BuildSAMLMetadataWithKey(
		"https://idp.example.com",
		"https://idp.example.com/sso/redirect",
		"https://idp.example.com/sso/post",
		nil,
	)
	assert.Contains(t, string(metadata), "https://idp.example.com")
	assert.NotContains(t, string(metadata), "KeyDescriptor")
}

func TestBuildSAMLMetadataWithKey_WithCert(t *testing.T) {
	metadata := BuildSAMLMetadataWithKey(
		"https://idp.example.com",
		"https://idp.example.com/sso/redirect",
		"https://idp.example.com/sso/post",
		nil,
	)
	assert.Contains(t, string(metadata), "EntityDescriptor")
	assert.Contains(t, string(metadata), "SingleSignOnService")
}
