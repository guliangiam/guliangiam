package users

import (
	"strings"
	"time"
)

// PasswordHistoryEntry records a previous password hash for reuse-prevention.
type PasswordHistoryEntry struct {
	ID           string
	UserID       string
	PasswordHash string
	PasswordAlgo string
	CreatedAt    time.Time
}

// PasswordVerificationAlgorithm returns the verifier for the historical hash.
// Like UserCredentials, an empty algorithm defaults to argon2id and an unknown
// marker is reported as not verifiable rather than guessed.
func (e *PasswordHistoryEntry) PasswordVerificationAlgorithm() (algorithm string, ok bool) {
	if e == nil || strings.TrimSpace(e.PasswordHash) == "" {
		return "", false
	}
	return localPasswordVerificationAlgorithm(e.PasswordAlgo)
}
