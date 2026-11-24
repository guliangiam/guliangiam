package users

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stretchr/testify/assert"
)

func TestRoutes_WebAuthnCredentialManagement(t *testing.T) {
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
		{"GetUserWebAuthnCredential", "GET", "/api/v1/users/{id}/webauthn-credentials/{credentialId}", "users:read"},
		{"RenameUserWebAuthnCredential", "POST", "/api/v1/users/{id}/webauthn-credentials/{credentialId}/rename", "users:write"},
		{"BeginUserWebAuthnCredentialRegistration", "POST", "/api/v1/users/{id}/webauthn-credentials/registration/begin", "users:write"},
		{"FinishUserWebAuthnCredentialRegistration", "POST", "/api/v1/users/{id}/webauthn-credentials/registration/finish", "users:write"},
	} {
		definition, ok := found[want.operationID]
		if !assert.True(t, ok, "missing route %s", want.operationID) {
			continue
		}
		assert.Equal(t, want.method, definition.Method)
		assert.Equal(t, want.pattern, definition.Pattern)
		assert.Equal(t, want.permission, definition.Permission)
		assert.Equal(t, route.SurfaceManagement, definition.Surface)
	}
}
