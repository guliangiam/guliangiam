package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterScope(t *testing.T) {
	scopes := []string{"openid", "profile", "email", "offline_access"}
	result := filterScope(scopes, "offline_access")
	assert.Equal(t, []string{"openid", "profile", "email"}, result)
}

func TestFilterScope_NotPresent(t *testing.T) {
	scopes := []string{"openid", "profile"}
	result := filterScope(scopes, "offline_access")
	assert.Equal(t, []string{"openid", "profile"}, result)
}

func TestFilterScope_Empty(t *testing.T) {
	result := filterScope([]string{}, "openid")
	assert.Empty(t, result)
}

func TestIntersectScopes(t *testing.T) {
	available := []string{"openid", "profile", "email"}
	requested := []string{"openid", "email", "phone"}
	result := intersectScopes(available, requested)
	assert.Equal(t, []string{"openid", "email"}, result)
}

func TestIntersectScopes_NoOverlap(t *testing.T) {
	available := []string{"openid", "profile"}
	requested := []string{"email", "phone"}
	result := intersectScopes(available, requested)
	assert.Empty(t, result)
}

func TestIntersectScopes_EmptyAvailable(t *testing.T) {
	result := intersectScopes([]string{}, []string{"openid"})
	assert.Empty(t, result)
}

func TestIntersectScopes_EmptyRequested(t *testing.T) {
	result := intersectScopes([]string{"openid"}, []string{})
	assert.Empty(t, result)
}

func TestContainsScope_Found(t *testing.T) {
	assert.True(t, containsScope([]string{"openid", "profile"}, "openid"))
}

func TestContainsScope_NotFound(t *testing.T) {
	assert.False(t, containsScope([]string{"openid", "profile"}, "email"))
}

func TestContainsScope_Empty(t *testing.T) {
	assert.False(t, containsScope([]string{}, "openid"))
}

func TestGenerateDeviceSecret(t *testing.T) {
	secret, err := generateDeviceSecret()
	assert.NoError(t, err)
	assert.NotEmpty(t, secret)
	// Should be hex-encoded 32 bytes = 64 chars
	assert.Len(t, secret, 64)
}

func TestGenerateDeviceSecret_Unique(t *testing.T) {
	s1, _ := generateDeviceSecret()
	s2, _ := generateDeviceSecret()
	assert.NotEqual(t, s1, s2)
}
