package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoginRequest_TrustedDeviceCredentialComesFromCookieOnly(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.AddCookie(&http.Cookie{Name: trustedDeviceCookieName(), Value: "cookie-credential"})

	command := LoginRequest{OrganizationSlug: "built-in", Username: "alice"}.toCommand(request)
	require.Equal(t, "cookie-credential", command.TrustedDeviceCredentialID)
}

func TestLoginRequest_WithoutCookieHasNoTrustedDeviceCredential(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	command := LoginRequest{OrganizationSlug: "built-in", Username: "alice"}.toCommand(request)
	require.Empty(t, command.TrustedDeviceCredentialID)
}
