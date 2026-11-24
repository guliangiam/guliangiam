package users

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stretchr/testify/assert"
)

// TestRoutes_SetUserPasswordUsesDedicatedPermission 固定权限分离：
// 口令重置必须是 users:password:reset，users:write 不再能改口令。
func TestRoutes_SetUserPasswordUsesDedicatedPermission(t *testing.T) {
	definitions := Routes()
	found := make(map[string]route.Definition, len(definitions))
	for _, definition := range definitions {
		found[definition.OperationID] = definition
	}

	definition, ok := found["SetUserPassword"]
	if !assert.True(t, ok, "missing route SetUserPassword") {
		return
	}
	assert.Equal(t, "POST", definition.Method)
	assert.Equal(t, "/api/v1/users/{id}/set-password", definition.Pattern)
	assert.Equal(t, "users:password:reset", definition.Permission)
	assert.Equal(t, route.SurfaceManagement, definition.Surface)
	assert.NotEqual(t, "users:write", definition.Permission, "users:write must no longer gate password changes")
}
