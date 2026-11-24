package auth

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stretchr/testify/assert"
)

func TestRoutes_RememberMe(t *testing.T) {
	definitions := Routes()
	found := make(map[string]route.Definition, len(definitions))
	for _, definition := range definitions {
		found[definition.OperationID] = definition
	}

	for _, want := range []struct {
		operationID string
		method      string
		pattern     string
		permission  string
	}{
		{"PostAuthRememberMeIssue", "POST", "/api/v1/auth/remember-me/issue", "auth.session:write"},
		{"PostAuthRememberMeExchange", "POST", "/api/v1/auth/remember-me/exchange", ""},
		{"PostAuthRememberMeRevoke", "POST", "/api/v1/auth/remember-me/revoke", "auth.session:write"},
	} {
		definition, ok := found[want.operationID]
		if !assert.True(t, ok, "missing route %s", want.operationID) {
			continue
		}
		assert.Equal(t, want.method, definition.Method)
		assert.Equal(t, want.pattern, definition.Pattern)
		assert.Equal(t, want.permission, definition.Permission)
		assert.Equal(t, route.SurfaceAuthentication, definition.Surface)
	}
}
