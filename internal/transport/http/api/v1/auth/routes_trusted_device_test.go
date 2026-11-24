package auth

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stretchr/testify/assert"
)

func TestRoutes_TrustedDevices(t *testing.T) {
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
		{"PostAuthTrustedDeviceBegin", "POST", "/api/v1/auth/trusted-devices/begin", "auth:trusted_device"},
		{"PostAuthTrustedDeviceRegister", "POST", "/api/v1/auth/trusted-devices", "auth:trusted_device"},
		{"GetAuthTrustedDevices", "GET", "/api/v1/auth/trusted-devices", "auth:trusted_device"},
		{"DeleteAuthTrustedDevice", "DELETE", "/api/v1/auth/trusted-devices/{deviceId}", "auth:trusted_device"},
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
