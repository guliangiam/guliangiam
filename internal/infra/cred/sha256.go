package cred

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SHA256Provider implements HashProvider using SHA-256 with salt.
// Format: {SHA256}salthex$hashhex
type SHA256Provider struct{}

// NewSHA256Provider creates a new SHA-256 hash provider.
func NewSHA256Provider() *SHA256Provider {
	return &SHA256Provider{}
}

// Hash generates a salted SHA-256 hash from the password.
func (p *SHA256Provider) Hash(password string) (string, error) {
	salt := generateSalt(16)
	saltHex := hex.EncodeToString(salt)
	hash := sha256.Sum256(append(salt, []byte(password)...))
	hashHex := hex.EncodeToString(hash[:])
	return fmt.Sprintf("{SHA256}%s$%s", saltHex, hashHex), nil
}

// Verify checks if the password matches the SHA-256 hash.
func (p *SHA256Provider) Verify(password, hash string) (bool, error) {
	inner := strings.TrimPrefix(hash, "{SHA256}")
	parts := strings.SplitN(inner, "$", 2)
	if len(parts) != 2 {
		return false, apperrors.NewInvalidArgument("invalid sha256 hash format")
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false, apperrors.WrapInternalf(err, "invalid sha256 salt")
	}

	computed := sha256.Sum256(append(salt, []byte(password)...))
	return hex.EncodeToString(computed[:]) == parts[1], nil
}

// Algorithm returns "sha256".
func (p *SHA256Provider) Algorithm() string {
	return AlgoSHA256
}

// Ensure SHA256Provider implements HashProvider.
var _ HashProvider = (*SHA256Provider)(nil)
