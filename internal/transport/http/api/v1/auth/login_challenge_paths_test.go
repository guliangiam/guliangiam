package auth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const loginChallengeUUID = "01924f00-0000-7000-8000-000000000001"

// TestIsTrulyPublicRequest_LoginChallengeReadMatchesRealSegment locks GAP-005:
// AC-PERM-004 keeps /api/v1/auth/login-challenges/* out of the anonymous
// whitelist, and the literal "{challengeId}" pattern must not match anything.
func TestIsTrulyPublicRequest_LoginChallengeReadMatchesRealSegment(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{"anonymous GET concrete uuid", http.MethodGet, "/api/v1/auth/login-challenges/" + loginChallengeUUID, false},
		{"anonymous POST concrete uuid", http.MethodPost, "/api/v1/auth/login-challenges/" + loginChallengeUUID, false},
		{"anonymous GET literal placeholder", http.MethodGet, "/api/v1/auth/login-challenges/{challengeId}", false},
		{"anonymous GET collection", http.MethodGet, "/api/v1/auth/login-challenges", false},
		{"anonymous GET empty segment", http.MethodGet, "/api/v1/auth/login-challenges/", false},
		{"anonymous GET extra segment", http.MethodGet, "/api/v1/auth/login-challenges/" + loginChallengeUUID + "/extra", false},
		{"anonymous GET approve path", http.MethodGet, "/api/v1/auth/login-challenges/" + loginChallengeUUID + "/approve", false},
		{"anonymous GET foreign prefix", http.MethodGet, "/api/v1/other/login-challenges/" + loginChallengeUUID, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isTrulyPublicRequest(tc.method, tc.path))
		})
	}
}

// TestAPIAuthorizationMiddleware_LoginChallengeRequiresSession verifies the
// GAP-005 decision: AC-PERM-004 forbids anonymous exemptions for
// /login-challenges/*, so the anonymous GET stops at Gate 2b (401). The
// password-login second factor completes through /api/v1/auth/login with a
// challengeId, so no pre-session poll depends on these endpoints; push MFA
// initiate/verify are independently session-required (see
// TestAPIAuthorizationMiddleware_PushMfaRequiresSession). A session (same or
// cross tenant) reaches the handler; object ownership is enforced in the
// application layer.
func TestAPIAuthorizationMiddleware_LoginChallengeRequiresSession(t *testing.T) {
	sessions := map[string]*SessionContext{
		"anonymous": nil,
		"same tenant": {
			UserID:           "01924f00-0000-7000-8000-000000000002",
			OrganizationID:   "org-uuid",
			OrganizationSlug: "acme",
			Username:         "alice",
			Source:           "repository",
		},
		"cross tenant": {
			UserID:           "01924f00-0000-7000-8000-0000000000ff",
			OrganizationID:   "other-org-uuid",
			OrganizationSlug: "other",
			Username:         "bob",
			Source:           "repository",
		},
	}
	for name, session := range sessions {
		t.Run(name+" GET", func(t *testing.T) {
			status, nextCalled, _ := serveAuthorizationRequest(t, http.MethodGet,
				"/api/v1/auth/login-challenges/"+loginChallengeUUID, session)
			if session == nil {
				assert.False(t, nextCalled, "anonymous challenge read must not reach the handler")
				assert.Equal(t, http.StatusUnauthorized, status)
				return
			}
			require.True(t, nextCalled, "%s challenge read must reach the handler", name)
			assert.Equal(t, http.StatusOK, status)
		})
	}

	for _, action := range []string{"approve", "reject"} {
		t.Run("anonymous "+action, func(t *testing.T) {
			status, nextCalled, _ := serveAuthorizationRequest(t, http.MethodPost,
				"/api/v1/auth/login-challenges/"+loginChallengeUUID+"/"+action, nil)
			assert.False(t, nextCalled, "anonymous %s must not reach the handler", action)
			assert.Equal(t, http.StatusUnauthorized, status)
		})
	}
}

// TestIsAuthRequiringSession_LoginChallengeIsExactSegment prevents the session
// gate from drifting back to "contains + suffix" matching; the bare challenge
// read and both actions are session-required.
func TestIsAuthRequiringSession_LoginChallengeIsExactSegment(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/api/v1/auth/login-challenges/" + loginChallengeUUID + "/approve", true},
		{"/api/v1/auth/login-challenges/" + loginChallengeUUID + "/reject", true},
		{"/api/v1/auth/login-challenges/" + loginChallengeUUID, true},
		{"/api/v1/auth/login-challenges/{challengeId}/approve", false},
		{"/api/v1/auth/login-challenges/{challengeId}", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.want, isAuthRequiringSession(tc.path))
		})
	}
}
