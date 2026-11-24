package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

func generateWebAuthnChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// FormatWebAuthnError formats a WebAuthn error for display.
func FormatWebAuthnError(operation string, err error) string {
	return fmt.Sprintf("webauthn %s failed: %v", operation, err)
}

// Ensure userdomain is used (import guard).
var _ = (*userdomain.User)(nil)
