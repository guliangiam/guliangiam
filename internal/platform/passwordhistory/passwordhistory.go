// Package passwordhistory 是口令历史哈希的唯一算法来源。
//
// 写入与校验必须调用同一组函数：历史行存的是 argon2id(pepper+password)，
// 不是用户凭据行里的口令哈希。历史上写入方存凭据哈希、校验方按
// pepper+明文 计算，两者永不相等，导致口令复用校验静默失效。
package passwordhistory

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"

	"github.com/alexedwards/argon2id"
	"github.com/guliangiam/guliangiam/internal/platform/config"
)

const (
	// Argon2IDPrefix is the encoded prefix of an argon2id hash.
	Argon2IDPrefix = "$argon2id$"

	// DefaultPepper is the fallback pepper used when PASSWORD_HISTORY_PEPPER is
	// unset. It is shared with config so production fail-fast and the runtime
	// fallback cannot drift.
	DefaultPepper = config.DefaultPasswordHistoryPepper

	argon2Memory      = 64 * 1024
	argon2Iterations  = 3
	argon2Parallelism = 2
	argon2SaltLength  = 16
	argon2KeyLength   = 32
)

// Pepper returns the pepper for password history hashing.
// It reads PASSWORD_HISTORY_PEPPER; if unset it falls back to the shared
// development sentinel and logs a warning. Thread-safe via sync.Once.
var Pepper = func() func() string {
	var (
		once sync.Once
		val  string
	)
	return func() string {
		once.Do(func() {
			// Defense in depth: startup validation should already have rejected
			// this. A binary that bypasses config.ValidateOrPanic must not silently
			// hash password history with the shared development sentinel.
			if issue := config.PasswordHistoryPepperIssue(); issue != "" {
				slog.Error("password history pepper rejected", "error", issue)
				panic(issue)
			}
			if p := config.PasswordHistoryPepper(); p != "" {
				val = p
				return
			}
			slog.Warn("PASSWORD_HISTORY_PEPPER environment variable not set; using default pepper. " +
				"Set PASSWORD_HISTORY_PEPPER for production deployments.")
			val = DefaultPepper
		})
		return val
	}
}()

// Hash hashes a plaintext password for storage in the password history table.
// The argon2id hash embeds its own random salt, so the same password produces
// a different hash on each call.
func Hash(password string) (string, error) {
	return argon2id.CreateHash(Pepper()+password, &argon2id.Params{
		Memory:      argon2Memory,
		Iterations:  argon2Iterations,
		Parallelism: argon2Parallelism,
		SaltLength:  argon2SaltLength,
		KeyLength:   argon2KeyLength,
	})
}

// Verify compares a plaintext password against a stored history hash.
// It supports both:
//   - Legacy SHA-256 hashes (64-char hex string, no prefix)
//   - Argon2id hashes (prefixed with "$argon2id$")
func Verify(password, storedHash string) (bool, error) {
	// Detect legacy SHA-256 hash: no argon2id prefix.
	if !strings.HasPrefix(storedHash, Argon2IDPrefix) {
		sum := sha256.Sum256([]byte(DefaultPepper + password))
		return hex.EncodeToString(sum[:]) == storedHash, nil
	}

	return argon2id.ComparePasswordAndHash(Pepper()+password, storedHash)
}

// LegacySHA256 returns the legacy peppered SHA-256 hash. It exists only for
// backward-compatible verification of old history entries.
//
// Deprecated: use Hash / Verify instead.
func LegacySHA256(password string) string {
	sum := sha256.Sum256([]byte(DefaultPepper + password))
	return hex.EncodeToString(sum[:])
}
