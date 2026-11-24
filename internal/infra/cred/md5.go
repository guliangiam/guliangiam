package cred

import (
	"crypto/md5" //nolint:gosec // G501: legacy hash verification for accounts created before argon2id
	"encoding/hex"
	"log/slog"
)

// MD5Provider implements HashProvider using unsalted MD5.
// WARNING: MD5 is cryptographically broken. This provider exists ONLY for
// verifying legacy hashes during migration. Never use it for new passwords.
type MD5Provider struct{}

// NewMD5Provider creates a new MD5 hash provider.
func NewMD5Provider() *MD5Provider {
	return &MD5Provider{}
}

// Hash generates an MD5 hash. This should NOT be used for new passwords.
func (p *MD5Provider) Hash(password string) (string, error) {
	slog.Warn("MD5 password hashing is deprecated; use argon2id or bcrypt for new passwords")
	hash := md5.Sum([]byte(password)) //nolint:gosec // G401: legacy compatibility only
	return hex.EncodeToString(hash[:]), nil
}

// Verify checks if the password matches the MD5 hash.
func (p *MD5Provider) Verify(password, hash string) (bool, error) {
	computed := md5.Sum([]byte(password)) //nolint:gosec // G401: verifying legacy hashes only
	return hex.EncodeToString(computed[:]) == hash, nil
}

// Algorithm returns "md5".
func (p *MD5Provider) Algorithm() string {
	return AlgoMD5
}

// Ensure MD5Provider implements HashProvider.
var _ HashProvider = (*MD5Provider)(nil)
