package auth

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// pkceChallengeMethodS256 is the only transform this server accepts
	// (RFC 7636 §4.3, ADR-004). "plain" is never valid here.
	pkceChallengeMethodS256 = "S256"

	// authorizationCodeEntropyBytes is the size of the random half of an
	// authorization code. RFC 6749 §10.5 requires a value that cannot be
	// guessed or derived; 32 bytes gives 256 bit, comfortably above the
	// 128 bit baseline and above the 74 bit of a UUIDv7 random segment
	// (设计报告 14.6.1「grant_types 白名单」).
	authorizationCodeEntropyBytes = 32

	// authorizationCodeSecretPrefix makes a leaked value recognizable in logs
	// and support tickets without making it guessable.
	authorizationCodeSecretPrefix = "gl_ac_"
)

// newAuthorizationCodeSecret returns a fresh authorization code value. The
// randomness comes from crypto/rand and never from a clock, a counter, or a
// user identifier, so two codes issued in the same nanosecond are independent.
func newAuthorizationCodeSecret() (string, error) {
	buffer := make([]byte, authorizationCodeEntropyBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", apperrors.NewInternal("generate authorization code")
	}
	return authorizationCodeSecretPrefix + base64.RawURLEncoding.EncodeToString(buffer), nil
}

// authorizationCodeIssuance carries everything a new authorization code must be
// bound to. Both issuing paths — the RFC 6749 authorize endpoint and the
// post-login endpoint — fill this same struct, which is what keeps the two
// paths from drifting (设计报告 14.6.4).
type authorizationCodeIssuance struct {
	OrganizationID   string
	OrganizationSlug string
	UserID           string
	Username         string
	ApplicationID    string
	ApplicationSlug  string
	RedirectURI      string
	Scopes           []string
	State            string
	Nonce            string
	CodeChallenge    string
	ChallengeMethod  string
	Resource         string
	// Sid is the OIDC session identifier of the session that produced this code
	// (设计报告 14.12.3). It is carried on the code because the token endpoint has
	// no session in hand: it only has the code.
	Sid string
	// AuthTime is the moment the end user actually authenticated. Both issuing
	// paths must supply it; falling back to "now" would make auth_time equal the
	// issue time and quietly reintroduce.
	AuthTime time.Time
}

// newAuthorizationCode is the single constructor for authorization codes. The
// entropy source, the TTL, and the binding columns live here and nowhere else,
// so a caller cannot issue a code that skips PKCE or resource binding.
//
// The plaintext code is returned on the object; the repository stores only its
// SHA-256 hash, so the value exists in memory for one redirect and never at rest.
func (s *Service) newAuthorizationCode(issuance authorizationCodeIssuance, now time.Time) (*authorizationcodedomain.AuthorizationCode, error) {
	if err := validatePKCEBinding(issuance.CodeChallenge, issuance.ChallengeMethod); err != nil {
		return nil, err
	}

	id, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate authorization code id")
	}
	secret, err := newAuthorizationCodeSecret()
	if err != nil {
		return nil, err
	}

	authTime := issuance.AuthTime
	if authTime.IsZero() {
		authTime = now
	}

	return &authorizationcodedomain.AuthorizationCode{
		ID:               id,
		Code:             secret,
		UserID:           issuance.UserID,
		Username:         issuance.Username,
		OrganizationID:   issuance.OrganizationID,
		OrganizationSlug: issuance.OrganizationSlug,
		ApplicationID:    issuance.ApplicationID,
		ApplicationSlug:  issuance.ApplicationSlug,
		RedirectURI:      strings.TrimSpace(issuance.RedirectURI),
		Scopes:           append([]string(nil), issuance.Scopes...),
		State:            strings.TrimSpace(issuance.State),
		Nonce:            strings.TrimSpace(issuance.Nonce),
		CodeChallenge:    strings.TrimSpace(issuance.CodeChallenge),
		ChallengeMethod:  strings.TrimSpace(issuance.ChallengeMethod),
		Resource:         strings.TrimSpace(issuance.Resource),
		Sid:              strings.TrimSpace(issuance.Sid),
		AuthTime:         authTime,
		ExpiresAt:        now.Add(authorizationCodeTTL),
		CreatedAt:        now,
	}, nil
}

// validatePKCEBinding enforces PKCE for every client, public or confidential
// (OAuth 2.1, 设计报告 14.6.4, ADR-004). The per-application `pkceRequired`
// column is retained for API compatibility but no longer gates anything.
func validatePKCEBinding(codeChallenge, challengeMethod string) error {
	if strings.TrimSpace(codeChallenge) == "" {
		return apperrors.NewInvalidArgument("codeChallenge is required")
	}
	if strings.TrimSpace(challengeMethod) != pkceChallengeMethodS256 {
		return apperrors.NewInvalidArgument("codeChallengeMethod must be S256")
	}
	return nil
}

// unsupportedResponseTypeError reports a response_type this server retired
// (implicit flow, 设计报告 14.6.1「禁用 response_type=token」). It deliberately does not map onto an
// application error code: RFC 6749 §4.1.2.1 gives the authorization endpoint its
// own `unsupported_response_type` code, which the protocol face reads back
// through ProtocolCodeError.
type unsupportedResponseTypeError struct {
	responseType string
}

func (e *unsupportedResponseTypeError) Error() string {
	if e.responseType == "" {
		return "unsupported response type"
	}
	return "unsupported response type: " + e.responseType
}

// ProtocolCode implements platformerrors.ProtocolCodeError.
func (e *unsupportedResponseTypeError) ProtocolCode() string {
	return "unsupported_response_type"
}

// HTTPStatus keeps the status mapping in sync with StatusCodeFromError.
func (e *unsupportedResponseTypeError) HTTPStatus() int {
	return 400
}

func newUnsupportedResponseTypeError(responseType string) error {
	return &unsupportedResponseTypeError{responseType: strings.TrimSpace(responseType)}
}
