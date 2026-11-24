package invitations

import (
	"crypto/rand"
	"encoding/base64"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// invitationCodeBytes is the CSPRNG entropy behind every invitation code.
// 16 bytes = 128 bits; the code is an independent secret and must never be
// derived from the row id (UUIDv7 ids embed a millisecond timestamp and only
// 4 random bits in their first 16 characters, which made codes enumerable by
// anyone who knew the creation millisecond — BE-A-01).
const invitationCodeBytes = 16

// generateInvitationCode returns a URL-safe code carrying invitationCodeBytes
// of CSPRNG entropy. base64.RawURLEncoding keeps the code safe for query
// strings and path segments without padding; the invitations.code column is
// TEXT (no width constraint) and the frontend passes the code as a query
// parameter, so the 22-character output is compatible with both.
func generateInvitationCode() (string, error) {
	buf := make([]byte, invitationCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", apperrors.WrapInternal(err, "generate invitation code")
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
