package users

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stretchr/testify/assert"
)

func TestRoutes_PlatformAdminTOTPRebind(t *testing.T) {
	definitions := Routes()
	found := make(map[string]route.Definition, len(definitions))
	for _, definition := range definitions {
		found[definition.OperationID] = definition
	}

	definition, ok := found["PostPlatformUserTOTPAdminRebind"]
	if !assert.True(t, ok, "missing route PostPlatformUserTOTPAdminRebind") {
		return
	}
	assert.Equal(t, "POST", definition.Method)
	assert.Equal(t, "/api/v1/platform/users/{id}/mfa/totp/admin-rebind", definition.Pattern)
	assert.Equal(t, "users:write", definition.Permission)
	assert.Equal(t, route.SurfaceManagement, definition.Surface)
	assert.Equal(t, 204, definition.SuccessStatus)
}
