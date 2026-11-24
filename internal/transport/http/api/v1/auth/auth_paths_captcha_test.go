package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const captchaProviderSensitiveBody = `{"id":"cp-1","type":"reCAPTCHA","clientId":"public-site-key","clientSecret":"secret-must-not-leak","clientSecretSet":true}`

func serveAuthorizationRequest(t *testing.T, method, path string, session *SessionContext) (status int, nextCalled bool, body string) {
	t.Helper()

	called := false
	handler := testAPIAuthorizationMiddleware(&baseAuthorizationPDPStub{allowed: true})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(captchaProviderSensitiveBody))
		}),
	)

	req := httptest.NewRequest(method, path, nil)
	if session != nil {
		req = req.WithContext(WithSessionContext(req.Context(), *session))
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder.Code, called, recorder.Body.String()
}

// TestAPIAuthorizationMiddleware_CaptchaProvidersRequireAuthentication is the
// middleware-level P0-2 regression: anonymous GET/POST/PATCH/DELETE on the
// provider administration surface must all stop at Gate 3 (401) and must not
// leak configuration fields.
func TestAPIAuthorizationMiddleware_CaptchaProvidersRequireAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "anonymous GET collection", method: http.MethodGet, path: "/api/v1/captcha-providers"},
		{name: "anonymous GET detail", method: http.MethodGet, path: "/api/v1/captcha-providers/cp-1"},
		{name: "anonymous POST", method: http.MethodPost, path: "/api/v1/captcha-providers"},
		{name: "anonymous PATCH", method: http.MethodPatch, path: "/api/v1/captcha-providers/cp-1"},
		{name: "anonymous DELETE", method: http.MethodDelete, path: "/api/v1/captcha-providers/cp-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, nextCalled, body := serveAuthorizationRequest(t, tc.method, tc.path, nil)
			assert.Equal(t, http.StatusUnauthorized, status)
			assert.False(t, nextCalled, "anonymous %s %s reached the handler", tc.method, tc.path)
			assert.NotContains(t, body, "clientSecret", "denial response leaked a configuration field")
			assert.NotContains(t, body, "secret-must-not-leak", "denial response leaked the secret value")
		})
	}
}

// TestAPIAuthorizationMiddleware_CaptchaProvidersAuthorizedStillPasses ensures
// the whitelist fix did not over-tighten: an authenticated principal that the
// PDP allows still reaches the management handler for every registered verb.
func TestAPIAuthorizationMiddleware_CaptchaProvidersAuthorizedStillPasses(t *testing.T) {
	session := &SessionContext{
		OrganizationID:   "org-uuid",
		OrganizationSlug: "acme",
		Username:         "alice",
		Source:           "repository",
	}
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "authorized GET detail", method: http.MethodGet, path: "/api/v1/captcha-providers/cp-1"},
		{name: "authorized POST", method: http.MethodPost, path: "/api/v1/captcha-providers"},
		{name: "authorized PATCH", method: http.MethodPatch, path: "/api/v1/captcha-providers/cp-1"},
		{name: "authorized DELETE", method: http.MethodDelete, path: "/api/v1/captcha-providers/cp-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, nextCalled, body := serveAuthorizationRequest(t, tc.method, tc.path, session)
			require.True(t, nextCalled, "authorized %s %s did not reach the handler", tc.method, tc.path)
			assert.Equal(t, http.StatusOK, status)
			assert.Contains(t, body, `"id":"cp-1"`)
		})
	}
}

// TestAPIAuthorizationMiddleware_LoginCaptchaReadStaysPublic prevents the
// security fix from breaking the actual anonymous login surface: the login
// page reads captcha status/configuration through the auth captcha endpoints.
func TestAPIAuthorizationMiddleware_LoginCaptchaReadStaysPublic(t *testing.T) {
	for _, path := range []string{
		"/api/v1/auth/captcha-status",
		"/api/v1/auth/captcha",
	} {
		t.Run(path, func(t *testing.T) {
			status, nextCalled, _ := serveAuthorizationRequest(t, http.MethodGet, path, nil)
			require.True(t, nextCalled, "pre-login captcha read %s was not treated as public", path)
			assert.Equal(t, http.StatusOK, status)
		})
	}
}
