package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoolPtr_True(t *testing.T) {
	p := boolPtr(true)
	assert.True(t, *p)
}

func TestBoolPtr_False(t *testing.T) {
	p := boolPtr(false)
	assert.False(t, *p)
}

func TestContainsString_Found(t *testing.T) {
	assert.True(t, containsString([]string{"a", "b", "c"}, "b"))
}

func TestContainsString_NotFound(t *testing.T) {
	assert.False(t, containsString([]string{"a", "b", "c"}, "d"))
}

func TestContainsString_Empty(t *testing.T) {
	assert.False(t, containsString([]string{}, "a"))
}

func TestBuildAuthorizationRedirectURL(t *testing.T) {
	url := buildAuthorizationRedirectURL("https://example.com/callback", "auth-code-123", "state-456")
	assert.Contains(t, url, "code=auth-code-123")
	assert.Contains(t, url, "state=state-456")
}

// TestBuildImplicitRedirectURL lives in pure_functions_coverage_test.go
// (table-driven, more assertions).

func TestUrlEncode_Plain(t *testing.T) {
	assert.Equal(t, "hello", urlEncode("hello"))
}

func TestUrlEncode_Space(t *testing.T) {
	assert.Equal(t, "hello%20world", urlEncode("hello world"))
}

func TestUrlEncode_Slash(t *testing.T) {
	assert.Equal(t, "a%2Fb", urlEncode("a/b"))
}

func TestIsUnreservedByte_Alpha(t *testing.T) {
	assert.True(t, isUnreservedByte('a'))
	assert.True(t, isUnreservedByte('Z'))
}

func TestIsUnreservedByte_Digit(t *testing.T) {
	assert.True(t, isUnreservedByte('0'))
	assert.True(t, isUnreservedByte('9'))
}

func TestIsUnreservedByte_Special(t *testing.T) {
	assert.True(t, isUnreservedByte('-'))
	assert.True(t, isUnreservedByte('.'))
	assert.True(t, isUnreservedByte('_'))
	assert.True(t, isUnreservedByte('~'))
}

func TestIsUnreservedByte_Reserved(t *testing.T) {
	assert.False(t, isUnreservedByte(' '))
	assert.False(t, isUnreservedByte('/'))
	assert.False(t, isUnreservedByte('+'))
}

func TestNormalizeScopesFromString_Spaced(t *testing.T) {
	result := normalizeScopesFromString("openid profile email")
	assert.Equal(t, []string{"openid", "profile", "email"}, result)
}

func TestNormalizeScopesFromString_EmptyInput(t *testing.T) {
	result := normalizeScopesFromString("")
	assert.Empty(t, result)
}

func TestCasGenerateTicketID(t *testing.T) {
	id1 := casGenerateTicketID("ST")
	id2 := casGenerateTicketID("ST")
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "ST-")
}

func TestRandomHex(t *testing.T) {
	h1 := randomHex(16)
	h2 := randomHex(16)
	assert.Len(t, h1, 32) // 16 bytes = 32 hex chars
	assert.NotEqual(t, h1, h2)
}

// TestGenerateDeviceCode lives in pure_functions_extra_test.go
// (also asserts uniqueness).
