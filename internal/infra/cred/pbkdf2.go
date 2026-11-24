package cred

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"golang.org/x/crypto/pbkdf2"
)

// PBKDF2Provider implements HashProvider using PBKDF2-HMAC-SHA256.
type PBKDF2Provider struct {
	Iterations int
	SaltSize   int
	KeyLen     int
}

// NewPBKDF2Provider creates a new PBKDF2 hash provider.
func NewPBKDF2Provider() *PBKDF2Provider {
	return &PBKDF2Provider{
		Iterations: 600000,
		SaltSize:   16,
		KeyLen:     32,
	}
}

// Hash generates a PBKDF2 hash from the password.
// Format: pbkdf2-sha256$i=iterations$l=saltBase64$keyBase64
func (p *PBKDF2Provider) Hash(password string) (string, error) {
	salt := generateSalt(p.SaltSize)
	key := pbkdf2.Key([]byte(password), salt, p.Iterations, p.KeyLen, sha256.New)
	return fmt.Sprintf("pbkdf2-sha256$i=%d$l=%s$%s",
		p.Iterations,
		base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(key),
	), nil
}

// Verify checks if the password matches the PBKDF2 hash.
func (p *PBKDF2Provider) Verify(password, hash string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 {
		return false, apperrors.NewInvalidArgument("invalid pbkdf2 hash format")
	}

	iterStr := strings.TrimPrefix(parts[1], "i=")
	iterations, err := strconv.Atoi(iterStr)
	if err != nil {
		return false, apperrors.WrapInternalf(err, "invalid pbkdf2 iterations")
	}

	saltStr := strings.TrimPrefix(parts[2], "l=")
	salt, err := base64.StdEncoding.DecodeString(saltStr)
	if err != nil {
		return false, apperrors.WrapInternalf(err, "invalid pbkdf2 salt")
	}

	keyStr := parts[3]
	expectedKey, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return false, apperrors.WrapInternalf(err, "invalid pbkdf2 key")
	}

	computedKey := pbkdf2.Key([]byte(password), salt, iterations, len(expectedKey), sha256.New)

	if len(computedKey) != len(expectedKey) {
		return false, nil
	}
	for i := range computedKey {
		if computedKey[i] != expectedKey[i] {
			return false, nil
		}
	}
	return true, nil
}

// Algorithm returns "pbkdf2".
func (p *PBKDF2Provider) Algorithm() string {
	return AlgoPBKDF2
}

// Ensure PBKDF2Provider implements HashProvider.
var _ HashProvider = (*PBKDF2Provider)(nil)
