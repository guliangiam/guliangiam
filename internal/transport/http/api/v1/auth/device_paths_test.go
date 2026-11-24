package auth

import (
	"net/http"
	"testing"
)

func TestDeviceFlowAnonymousBoundary(t *testing.T) {
	public := []string{"/api/v1/auth/device/authorize", "/api/v1/auth/device/token"}
	for _, path := range public {
		if !isTrulyPublicRequest(http.MethodPost, path) {
			t.Fatalf("%s must be anonymous for RFC 8628", path)
		}
	}
	sessionBound := []string{"/api/v1/auth/device/verify", "/api/v1/auth/device/cancel"}
	for _, path := range sessionBound {
		if isTrulyPublicRequest(http.MethodPost, path) {
			t.Fatalf("%s must require a session", path)
		}
		if !isAuthRequiringSession(path) {
			t.Fatalf("%s must be session-required", path)
		}
	}
}
