package auth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFaceIDCompleteIsAnonymousHandshake pins H12: the three-step FaceID login
// chain is anonymous end to end — the session is issued by complete itself,
// aligned with qrcode exchange and webauthn authentication finish.
func TestFaceIDCompleteIsAnonymousHandshake(t *testing.T) {
	for _, path := range []string{
		"/api/v1/auth/faceid",
		"/api/v1/auth/faceid/verify",
		"/api/v1/auth/faceid/complete",
	} {
		t.Run(path, func(t *testing.T) {
			assert.True(t, isTrulyPublicRequest(http.MethodPost, path),
				"%s is a login handshake and must be anonymous", path)
			assert.True(t, isAuthLoginFlowPublic(path))
			assert.False(t, isAuthRequiringSession(path),
				"%s must not demand a pre-existing session", path)
		})
	}
	// The self-service plural faceids surface must not ride the handshake.
	assert.False(t, isTrulyPublicRequest(http.MethodPost, "/api/v1/auth/faceids"))
	assert.True(t, isAuthRequiringSession("/api/v1/auth/faceids"))
}

// TestPasswordResetIsAnonymousHandshake pins R2-B01 (Q2 裁定方案 A): the
// front-end forgot-password page only calls POST /api/v1/auth/password/reset;
// both tracks of the reset handshake must stay anonymous end to end.
func TestPasswordResetIsAnonymousHandshake(t *testing.T) {
	for _, path := range []string{
		"/api/v1/auth/password/reset",
		"/api/v1/auth/reset-password",
	} {
		t.Run(path, func(t *testing.T) {
			assert.True(t, isTrulyPublicRequest(http.MethodPost, path),
				"%s is a reset handshake and must be anonymous", path)
			assert.True(t, isAuthLoginFlowPublic(path))
			assert.False(t, isAuthRequiringSession(path),
				"%s must not demand a pre-existing session", path)
		})
	}
	// The session-bound password surfaces must not ride the handshake.
	assert.False(t, isAuthLoginFlowPublic("/api/v1/auth/password/change"))
	assert.False(t, isAuthLoginFlowPublic("/api/v1/auth/password/check"))
	assert.True(t, isAuthRequiringSession("/api/v1/auth/password/change"))
	assert.True(t, isAuthRequiringSession("/api/v1/auth/password/check"))
}

// TestCaptchaRuntimeMatchesPublicRule pins BE-10: the runtime anonymous gate
// enumerates the same three exact captcha paths as PublicRule; any other
// captcha-prefixed shape is no longer silently anonymous.
func TestCaptchaRuntimeMatchesPublicRule(t *testing.T) {
	for _, path := range []string{
		"/api/v1/auth/captcha",
		"/api/v1/auth/captcha-status",
		"/api/v1/auth/captcha/verify",
	} {
		t.Run("exact "+path, func(t *testing.T) {
			assert.True(t, isAuthLoginFlowPublic(path))
			rule, kind := PublicRule(path)
			assert.Equal(t, PublicRuleExact, kind)
			assert.Equal(t, "exact:"+path, rule)
		})
	}
	for _, path := range []string{
		"/api/v1/auth/captcha/extra",
		"/api/v1/auth/captcha-status/extra",
		"/api/v1/auth/captchas",
	} {
		t.Run("family "+path, func(t *testing.T) {
			assert.False(t, isAuthLoginFlowPublic(path),
				"%s must not ride an exact captcha decision", path)
			assert.False(t, isTrulyPublicRequest(http.MethodGet, path),
				"%s must not be anonymous at runtime", path)
			_, kind := PublicRule(path)
			assert.Equal(t, PublicRuleFamily, kind,
				"%s stays a family match that must self-declare", path)
		})
	}
}

// TestSessionExemptionsAreExactPaths pins BE-11: the MFA enable/disable and
// qrcode confirm session exemptions match only the registered route shapes,
// not arbitrary Contains/suffix lookalikes.
func TestSessionExemptionsAreExactPaths(t *testing.T) {
	for _, path := range []string{
		"/api/v1/auth/mfa/push/enable",
		"/api/v1/auth/mfa/push/disable",
		"/api/v1/auth/qrcode/confirm",
	} {
		t.Run("registered "+path, func(t *testing.T) {
			assert.True(t, isAuthRequiringSession(path))
			assert.False(t, isTrulyPublicRequest(http.MethodPost, path))
		})
	}
	for _, path := range []string{
		"/api/v1/auth/anything/mfa/whatever/enable",
		"/api/v1/auth/anything/mfa/whatever/disable",
		"/api/v1/auth/mfa/push/enable/extra",
		"/api/v1/auth/qrcode/confirm/extra",
		"/api/v1/auth/anything/qrcode/confirm",
	} {
		t.Run("lookalike "+path, func(t *testing.T) {
			assert.False(t, isAuthRequiringSession(path),
				"%s must not inherit a session exemption", path)
		})
	}
}
