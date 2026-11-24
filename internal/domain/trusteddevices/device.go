package trusteddevices

import (
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TrustedDevice is a user-approved device that may skip MFA within its trust
// window. Trust is bound to the device credential and the User-Agent hash
// observed at registration — never to an IP address.
type TrustedDevice struct {
	ID             string
	OrganizationID string
	UserID         string
	// DeviceCredentialID is the SHA-256 hex digest (64 lowercase chars) of the
	// client device credential; never plaintext.
	DeviceCredentialID string
	DevicePublicKey    []byte
	UAHash             string
	CreatedAt          time.Time
	LastSeenAt         time.Time
	ExpiresAt          time.Time
	RevokedAt          *time.Time
	Revision           int64
	UpdatedAt          time.Time
}

const (
	// credentialDigestLength is the length of a lowercase hex SHA-256 digest.
	credentialDigestLength = 64
	// minCredentialLength is the floor for a base64url credential. A 32-char
	// base64url string carries 6 bits per character, i.e. 192 bits at the full
	// alphabet — comfortably above the 128-bit client contract. The floor is
	// deliberately above 128 bits so partial-alphabet or biased generators still
	// have margin, and one simple threshold covers both accepted forms. Length
	// and charset are a floor only: the server cannot prove entropy.
	minCredentialLength = 32
)

// ValidateCredentialInput validates the plaintext credential a client submits
// before it is hashed. Accepted forms:
//   - 64 hex characters (256 bits), or
//   - base64url (A-Z a-z 0-9 - _) with at least 32 characters (>=192 bits).
//
// Anything else is rejected as 422 unprocessable_entity: the request bound, but
// the value is outside the accepted format. The value itself is never echoed in
// the error metadata (a plaintext credential must not reach a response body).
// This is the same normalization entry point used by the digest function, so
// every stored credential passes through it.
func ValidateCredentialInput(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return apperrors.NewInvalidArgument("deviceCredentialId is required")
	}
	if len(value) == credentialDigestLength && isLowerHex(value) {
		return nil
	}
	if len(value) >= minCredentialLength && isBase64URL(value) {
		return nil
	}
	return invalidCredentialFormat()
}

// ValidateCredentialDigest accepts only the persisted form of a device
// credential: 64 lowercase hexadecimal characters. Persistence boundaries call
// it before writing so a plaintext credential can never reach the database,
// even from a caller that bypasses the application layer.
func ValidateCredentialDigest(value string) error {
	if len(value) != credentialDigestLength || !isLowerHex(value) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"deviceCredentialId must be a SHA-256 hex digest",
			map[string]any{"field": "deviceCredentialId", "format": "sha256_hex_digest"},
		)
	}
	return nil
}

func invalidCredentialFormat() error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"deviceCredentialId must be 64 hex characters or at least 32 base64url characters",
		map[string]any{
			"field":  "deviceCredentialId",
			"format": "sha256_hex_digest_or_base64url",
			"min":    minCredentialLength,
		},
	)
}

func isLowerHex(value string) bool {
	for _, c := range value {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

func isBase64URL(value string) bool {
	for _, c := range value {
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '_':
		default:
			return false
		}
	}
	return true
}

// Active reports whether the device is still trusted at now: not revoked and
// not past its hard expiration.
func (d *TrustedDevice) Active(now time.Time) bool {
	return d != nil && d.RevokedAt == nil && d.ExpiresAt.After(now)
}
