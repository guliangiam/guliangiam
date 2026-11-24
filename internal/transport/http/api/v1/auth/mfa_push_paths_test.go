package auth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var pushMfaPaths = []string{
	"/api/v1/auth/mfa/push/initiate",
	"/api/v1/auth/mfa/push/verify",
}

// TestPushMfaPathsAreSessionRequired locks the E1-L5b classification. Both
// endpoints act on the caller's own session, so they must not ride the
// login-flow whitelist. The password-login second factor is completed through
// /api/v1/auth/login with a challengeId and stays anonymous.
func TestPushMfaPathsAreSessionRequired(t *testing.T) {
	for _, path := range pushMfaPaths {
		t.Run(path, func(t *testing.T) {
			assert.False(t, isTrulyPublicRequest(http.MethodPost, path),
				"%s must not be anonymous", path)
			assert.False(t, isAuthLoginFlowPublic(path),
				"%s must not ride the login-flow whitelist", path)
			assert.True(t, isAuthRequiringSession(path),
				"%s must require a session", path)
		})
	}
}

// TestAPIAuthorizationMiddleware_PushMfaRequiresSession proves anonymous calls
// stop at Gate 2b before the application layer. An authenticated session still
// reaches the handler, where E1-L5 binds the subject and rejects foreign
// challenges with 401 (TestPushMfaVerifyRejectsForeignChallenge).
func TestAPIAuthorizationMiddleware_PushMfaRequiresSession(t *testing.T) {
	session := &SessionContext{
		UserID:           "01924f00-0000-7000-8000-000000000002",
		OrganizationID:   "org-uuid",
		OrganizationSlug: "acme",
		Username:         "alice",
		Source:           "repository",
	}
	for _, path := range pushMfaPaths {
		t.Run("anonymous "+path, func(t *testing.T) {
			status, nextCalled, _ := serveAuthorizationRequest(t, http.MethodPost, path, nil)
			assert.Equal(t, http.StatusUnauthorized, status)
			assert.False(t, nextCalled, "an anonymous request must not reach the handler")
		})
		t.Run("authenticated "+path, func(t *testing.T) {
			status, nextCalled, _ := serveAuthorizationRequest(t, http.MethodPost, path, session)
			assert.Equal(t, http.StatusOK, status)
			assert.True(t, nextCalled,
				"an authenticated session must reach the handler; ownership is enforced there")
		})
	}
}

// TestIsAuthRequiringSession_PushMfaMatchesExactPaths prevents the session gate
// from widening to a /mfa/ prefix rule.
func TestIsAuthRequiringSession_PushMfaMatchesExactPaths(t *testing.T) {
	for _, path := range []string{
		"/api/v1/auth/mfa/push/initiate/extra",
		"/api/v1/auth/mfa/push/verify/extra",
	} {
		t.Run(path, func(t *testing.T) {
			assert.False(t, isAuthRequiringSession(path),
				"only the exact push MFA paths are classified as session-required")
		})
	}
}
