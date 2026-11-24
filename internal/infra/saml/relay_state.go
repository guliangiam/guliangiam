package saml

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RelayState envelope constants.
//
// The envelope is `v1.<issuedAtUnix>.<hexHMAC>.<originalRelayState>`. Keeping the
// original value last lets the console strip the envelope for display without
// knowing the key, while only the server can produce a valid signature.
const (
	relayStateVersion   = "v1"
	relayStateKeyLabel  = "guliangiam:saml-relay-state:v1"
	relayStateTTL       = 10 * time.Minute
	relayStateClockSkew = 60 * time.Second
	relayStateMaxLength = 4096
	relayStatePartCount = 4
)

// ErrInvalidRelayState covers every rejection reason so callers can map the
// failure to a single 400 without leaking which check failed.
var ErrInvalidRelayState = errors.New("invalid relay state")

// RelayStateSigner signs SAML RelayState with a server-held key.
//
// The key is derived from ENCRYPTION_KEY_CURRENT with domain separation, so no
// additional deployment secret is required and the browser never sees it.
type RelayStateSigner struct {
	key  []byte
	ttl  time.Duration
	skew time.Duration
	now  func() time.Time
}

// NewRelayStateSigner derives the RelayState signing subkey from masterKey.
//
// masterKey is the raw environment value; base64-encoded 32-byte keys (the
// format ENCRYPTION_KEY_CURRENT uses) are decoded first so the derived subkey
// has full entropy.
func NewRelayStateSigner(masterKey string) (*RelayStateSigner, error) {
	masterKey = strings.TrimSpace(masterKey)
	if masterKey == "" {
		return nil, errors.New("saml relay state signer: master key is empty")
	}

	material := []byte(masterKey)
	if decoded, err := base64.StdEncoding.DecodeString(masterKey); err == nil && len(decoded) >= 32 {
		material = decoded
	}

	mac := hmac.New(sha256.New, material)
	mac.Write([]byte(relayStateKeyLabel))

	return &RelayStateSigner{
		key:  mac.Sum(nil),
		ttl:  relayStateTTL,
		skew: relayStateClockSkew,
		now:  time.Now,
	}, nil
}

// Sign returns `v1.<issuedAtUnix>.<hexHMAC>.<relayState>` bound to the given
// organization and application.
func (s *RelayStateSigner) Sign(relayState, organizationSlug, applicationSlug string) (string, error) {
	if relayState == "" {
		return "", fmt.Errorf("%w: empty state", ErrInvalidRelayState)
	}
	if len(relayState) > relayStateMaxLength {
		return "", fmt.Errorf("%w: state too long", ErrInvalidRelayState)
	}

	issuedAt := s.now().UTC().Unix()
	signature := s.signature(relayState, organizationSlug, applicationSlug, issuedAt)

	return fmt.Sprintf("%s.%d.%s.%s",
		relayStateVersion,
		issuedAt,
		hex.EncodeToString(signature),
		relayState,
	), nil
}

// Verify checks the envelope and returns the original RelayState.
func (s *RelayStateSigner) Verify(signed, organizationSlug, applicationSlug string) (string, error) {
	parts := strings.SplitN(signed, ".", relayStatePartCount)
	if len(parts) != relayStatePartCount {
		return "", fmt.Errorf("%w: malformed envelope", ErrInvalidRelayState)
	}
	if parts[0] != relayStateVersion {
		return "", fmt.Errorf("%w: unsupported version", ErrInvalidRelayState)
	}

	issuedAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", fmt.Errorf("%w: malformed issue time", ErrInvalidRelayState)
	}

	relayState := parts[3]
	if relayState == "" || len(relayState) > relayStateMaxLength {
		return "", fmt.Errorf("%w: invalid state payload", ErrInvalidRelayState)
	}

	provided, err := hex.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("%w: malformed signature", ErrInvalidRelayState)
	}

	expected := s.signature(relayState, organizationSlug, applicationSlug, issuedAt)
	if !hmac.Equal(provided, expected) {
		return "", fmt.Errorf("%w: signature mismatch", ErrInvalidRelayState)
	}

	now := s.now().UTC()
	issued := time.Unix(issuedAt, 0).UTC()
	if issued.After(now.Add(s.skew)) {
		return "", fmt.Errorf("%w: issue time in the future", ErrInvalidRelayState)
	}
	if now.After(issued.Add(s.ttl + s.skew)) {
		return "", fmt.Errorf("%w: expired", ErrInvalidRelayState)
	}

	return relayState, nil
}

// signature binds the version, org, app, issue time and original state into one
// MAC. NUL separators keep the fields unambiguous.
func (s *RelayStateSigner) signature(relayState, organizationSlug, applicationSlug string, issuedAt int64) []byte {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(relayStateVersion))
	mac.Write([]byte{0})
	mac.Write([]byte(organizationSlug))
	mac.Write([]byte{0})
	mac.Write([]byte(applicationSlug))
	mac.Write([]byte{0})
	mac.Write([]byte(strconv.FormatInt(issuedAt, 10)))
	mac.Write([]byte{0})
	mac.Write([]byte(relayState))
	return mac.Sum(nil)
}
