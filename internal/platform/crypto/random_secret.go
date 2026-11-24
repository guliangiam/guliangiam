package crypto

import (
	"crypto/rand"
	"encoding/base64"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SecretEntropyBytes is the CSPRNG output size for an opaque secret this package
// mints. 32 bytes is 256 bits, which is the floor the design report sets for
// every value an attacker could otherwise enumerate (授权码 14.6.4, 会话 sid 14.12.3).
const SecretEntropyBytes = 32

// NewRandomSecret returns prefix followed by nBytes of crypto/rand output, encoded
// without padding (base64url). The prefix keeps the value identifiable in logs and
// support tickets without revealing anything about it.
//
// It exists so the authorization code and the session sid cannot drift apart:
// both used to compute their own entropy, and was exactly a code path that
// drew from a UUIDv7 instead of crypto/rand (74 bits of randomness in a field the
// document requires to be at least 128).
func NewRandomSecret(prefix string, nBytes int) (string, error) {
	if nBytes <= 0 {
		nBytes = SecretEntropyBytes
	}
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		// A CSPRNG failure is not a caller problem: refusing is the only safe
		// answer, because a low-entropy secret is indistinguishable from a
		// leaked one.
		return "", apperrors.WrapInternal(err, "generate random secret")
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buf), nil
}
