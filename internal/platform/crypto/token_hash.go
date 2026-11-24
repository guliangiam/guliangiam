package crypto

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// HashTokenValue computes a SHA-256 hash of a token value and returns it as a hex-encoded string.
// This is used for storing token hashes in the database while returning the raw value to the caller.
func HashTokenValue(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// VerifyTokenValue reports whether a presented token hashes to the stored hash.
//
// Compare-and-hash, not hash-and-lookup: when the stored hash is already in hand
// (the row was fetched by another key), the comparison must not short-circuit on
// the first differing byte, or the response time leaks how much of a guessed
// token was correct. subtle.ConstantTimeCompare gives that guarantee; a plain
// `==` on strings does not.
//
// An empty stored hash never matches: it means "no credential was ever issued",
// which must not be read as "any credential is acceptable".
func VerifyTokenValue(presented, storedHash string) bool {
	if presented == "" || storedHash == "" {
		return false
	}
	sum := sha256.Sum256([]byte(presented))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(storedHash)) == 1
}
