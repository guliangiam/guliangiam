package auth

import (
	"errors"
	"testing"

	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
)

func TestWebAuthnFailureReason(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"challenge mismatch", platformwebauthn.ErrChallengeMismatch, "challenge_mismatch"},
		{"origin not allowed", platformwebauthn.ErrOriginNotAllowed, "origin_not_allowed"},
		{"rp id mismatch", platformwebauthn.ErrRPIDMismatch, "rp_id_mismatch"},
		{"user presence required", platformwebauthn.ErrUserPresenceRequired, "user_presence_required"},
		{"user verification required", platformwebauthn.ErrUserVerificationRequired, "user_verification_required"},
		{"sign count regression", platformwebauthn.ErrSignCountRegression, "sign_count_regression"},
		{"signature invalid", platformwebauthn.ErrSignatureInvalid, "signature_invalid"},
		{"malformed assertion", platformwebauthn.ErrMalformedAssertion, "malformed_assertion"},
		{"unknown error", errors.New("boom"), "assertion_invalid"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := webAuthnFailureReason(tc.err); got != tc.want {
				t.Fatalf("webAuthnFailureReason(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}
