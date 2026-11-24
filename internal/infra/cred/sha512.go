package cred

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SHA512Provider implements HashProvider using SHA-512 with salt.
// Format: {SHA512}salthex$hashhex
type SHA512Provider struct{}

// NewSHA512Provider creates a new SHA-512 hash provider.
func NewSHA512Provider() *SHA512Provider {
	return &SHA512Provider{}
}

// Hash generates a salted SHA-512 hash from the password.
func (p *SHA512Provider) Hash(password string) (string, error) {
	salt := generateSalt(16)
	saltHex := hex.EncodeToString(salt)
	hash := sha512.Sum512(append(salt, []byte(password)...))
	hashHex := hex.EncodeToString(hash[:])
	return fmt.Sprintf("{SHA512}%s$%s", saltHex, hashHex), nil
}

// Verify checks if the password matches the SHA-512 hash.
func (p *SHA512Provider) Verify(password, hash string) (bool, error) {
	inner := strings.TrimPrefix(hash, "{SHA512}")
	parts := strings.SplitN(inner, "$", 2)
	if len(parts) != 2 {
		return false, apperrors.NewInvalidArgument("invalid sha512 hash format")
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false, apperrors.WrapInternalf(err, "invalid sha512 salt")
	}

	computed := sha512.Sum512(append(salt, []byte(password)...))
	return hex.EncodeToString(computed[:]) == parts[1], nil
}

// Algorithm returns "sha512".
func (p *SHA512Provider) Algorithm() string {
	return AlgoSHA512
}

// Ensure SHA512Provider implements HashProvider.
var _ HashProvider = (*SHA512Provider)(nil)
