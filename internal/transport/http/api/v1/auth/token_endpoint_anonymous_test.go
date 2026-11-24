package auth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var clientAuthenticatedProtocolEndpointPaths = []string{
	"/api/v1/auth/tokens/exchange",
	"/api/v1/auth/tokens/jwt-bearer",
	"/api/v1/auth/tokens/token-exchange",
	"/api/v1/auth/tokens/introspect",
	"/api/v1/auth/tokens/client-credentials",
	"/api/v1/auth/oauth2/revoke",
	"/api/v1/auth/oauth2/introspect",
}

// TestClientAuthenticatedTokenEndpointsAreAnonymous locks the reachability fix:
// every endpoint that authenticates the client itself must (a) be matched by
// the shared predicate for POST, (b) not be classified as session-requiring,
// and (c) actually reach the handler without a session.
func TestClientAuthenticatedTokenEndpointsAreAnonymous(t *testing.T) {
	for _, path := range clientAuthenticatedProtocolEndpointPaths {
		t.Run(path, func(t *testing.T) {
			assert.True(t, IsClientAuthenticatedProtocolEndpoint(http.MethodPost, path))
			assert.True(t, isTrulyPublicRequest(http.MethodPost, path))
			assert.False(t, isAuthRequiringSession(path),
				"a session-bound classification would make the OAuth2 protocol call unreachable")

			status, nextCalled, _ := serveAuthorizationRequest(t, http.MethodPost, path, nil)
			assert.True(t, nextCalled, "an anonymous client must reach the protocol handler")
			assert.Equal(t, http.StatusOK, status)
		})
	}
}

// TestClientAuthenticatedProtocolEndpointPredicateIsExact prevents the
// predicate from widening into a prefix rule: only the exact POST patterns
// count.
func TestClientAuthenticatedProtocolEndpointPredicateIsExact(t *testing.T) {
	for _, path := range clientAuthenticatedProtocolEndpointPaths {
		for _, method := range []string{
			http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodDelete,
		} {
			assert.False(t, IsClientAuthenticatedProtocolEndpoint(method, path),
				"%s %s must not ride the client-authenticated exemption", method, path)
		}

		for _, variant := range []string{
			path + "/",
			path + "/extra",
			path[:len(path)-1],
		} {
			assert.False(t, IsClientAuthenticatedProtocolEndpoint(http.MethodPost, variant),
				"only the exact pattern is exempt, got %s", variant)
		}
	}

	assert.False(t, IsClientAuthenticatedProtocolEndpoint(http.MethodPost, "/api/v1/auth/tokens"),
		"the bare namespace is not a token endpoint")
	assert.False(t, IsClientAuthenticatedProtocolEndpoint(http.MethodPost, "/api/v1/auth/tokens/unknown"),
		"an unknown token endpoint must not inherit the exemption")

	// The rest of the /oauth2/ namespace stays session-bound: only revoke and
	// introspect authenticate the client itself.
	for _, path := range []string{
		"/api/v1/auth/oauth2/authorize",
		"/api/v1/auth/oauth2/userinfo",
		"/api/v1/auth/oauth2/logout",
		"/api/v1/auth/oauth2/unknown",
	} {
		assert.False(t, IsClientAuthenticatedProtocolEndpoint(http.MethodPost, path),
			"%s must stay on the session-bound /oauth2/ prefix", path)
		assert.True(t, isAuthRequiringSession(path),
			"%s must keep requiring a session", path)
	}
}

// TestSessionBoundAuthEndpointsStayGated is the counterpart negative case: the
// user-mediated steps of the authorization-code flow still require a session
// (and therefore the password gate), so releasing the token endpoints must not
// release /authorize.
func TestSessionBoundAuthEndpointsStayGated(t *testing.T) {
	for _, path := range []string{
		"/api/v1/auth/authorize",
		"/api/v1/auth/account",
		"/api/v1/auth/consents/grant",
	} {
		t.Run(path, func(t *testing.T) {
			assert.False(t, isTrulyPublicRequest(http.MethodPost, path))
			assert.True(t, isAuthRequiringSession(path))

			status, nextCalled, _ := serveAuthorizationRequest(t, http.MethodPost, path, nil)
			assert.False(t, nextCalled)
			assert.Equal(t, http.StatusUnauthorized, status)
		})
	}
}
