package cred

import "crypto/rand"

// generateSalt generates a random salt of the given size.
func generateSalt(size int) []byte {
	salt := make([]byte, size)
	_, _ = rand.Read(salt)
	return salt
}
