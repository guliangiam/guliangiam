package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/passwordhistory"
)

const (
	// DefaultPasswordHistoryCount is the default number of previous passwords to check against.
	DefaultPasswordHistoryCount = 5

	// defaultPasswordHistoryPepper is the fallback pepper used when the
	// PASSWORD_HISTORY_PEPPER environment variable is not set. It is shared
	// with config so production fail-fast and the runtime fallback cannot drift.
	defaultPasswordHistoryPepper = passwordhistory.DefaultPepper

	// argon2idPrefix is the prefix of argon2id encoded hashes.
	argon2idPrefix = passwordhistory.Argon2IDPrefix
)

// passwordHistoryPepper returns the pepper for password history hashing.
func passwordHistoryPepper() string { return passwordhistory.Pepper() }

// passwordHistoryReuseError is the single 422 shape for a history rejection.
// It carries a machine-readable rule and a failures list, so clients can render
// every failed item instead of a generic "password invalid" sentence.
func passwordHistoryReuseError(historyCount int) error {
	message := fmt.Sprintf("password cannot be reused (checked against last %d passwords)", historyCount)
	return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, message, map[string]any{
		"field":        "password",
		"rule":         "password_history",
		"historyCount": historyCount,
		"failures": []map[string]any{{
			"field":   "password",
			"rule":    "password_history",
			"message": message,
		}},
	})
}

// CheckPasswordHistoryCommand contains parameters for checking password history.
type CheckPasswordHistoryCommand struct {
	UserID   string
	Password string
	Limit    int // Number of historical passwords to check (0 = use default)
}

// CheckPasswordHistory checks whether the given password has been used recently.
// It returns an error if the password matches any of the user's recent passwords.
func (s *Service) CheckPasswordHistory(ctx context.Context, command CheckPasswordHistoryCommand) error {
	if strings.TrimSpace(command.UserID) == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if strings.TrimSpace(command.Password) == "" {
		return apperrors.NewInvalidArgument("password is required")
	}

	limit := command.Limit
	if limit <= 0 {
		limit = DefaultPasswordHistoryCount
	}

	// Retrieve password history.
	history, err := s.UOW.Users().GetPasswordHistory(ctx, command.UserID, limit)
	if err != nil {
		return err
	}

	// Check against each historical entry using verification (supports both
	// legacy SHA-256 and new argon2id hashes).
	for _, entry := range history {
		matched, err := verifyPasswordHistoryHash(command.Password, entry.PasswordHash)
		if err != nil {
			slog.WarnContext(ctx, "failed to verify password history entry", "error", err)
			continue
		}
		if matched {
			return passwordHistoryReuseError(limit)
		}
	}

	return nil
}

// createPasswordHistoryHash hashes a password for storage in the password
// history using argon2id with a pepper prefix. The argon2id hash embeds its own
// random salt and parameters, so the same password produces different hashes.
func createPasswordHistoryHash(password string) (string, error) {
	return passwordhistory.Hash(password)
}

// verifyPasswordHistoryHash verifies a plaintext password against a stored hash.
// It supports both:
//   - Legacy SHA-256 hashes (64-char hex string, no prefix)
//   - New argon2id hashes (prefixed with "$argon2id$")
func verifyPasswordHistoryHash(password, storedHash string) (bool, error) {
	return passwordhistory.Verify(password, storedHash)
}

// hashPasswordForHistorySHA256 is the legacy SHA-256 hashing function kept for
// backward-compatible verification of old history entries.
//
// Deprecated: Use createPasswordHistoryHash / verifyPasswordHistoryHash instead.
func hashPasswordForHistorySHA256(password string) string {
	return passwordhistory.LegacySHA256(password)
}

// passwordHistoryStore is the only capability the reuse check needs. Keeping
// it narrow lets the check run on either the pool-level UoW (CPU/read phase
// outside a transaction) or a transaction-scoped UoW.
type passwordHistoryStore interface {
	Users() userdomain.Repository
}

// ValidatePasswordNotReused validates a new password is not in the user's
// recent history, using the given store context.
func ValidatePasswordNotReused(ctx context.Context, store passwordHistoryStore, userID, newPassword string, limit int) error {
	if limit <= 0 {
		limit = DefaultPasswordHistoryCount
	}
	history := fetchPasswordHistoryForReuse(ctx, store.Users(), userID, limit)
	return checkPasswordHistoryEntries(history, newPassword, limit)
}

// fetchPasswordHistoryForReuse reads history entries with the deliberate
// fail-open policy: password history is a policy add-on, not an authentication
// boundary. When the store is unavailable (missing migration, query failure,
// database hiccup) blocking the change would leave an enforced user unable to
// clear need_update_password — i.e. locked out with no self-service path. The
// worst degradation is one reused password; the event must stay alertable.
func fetchPasswordHistoryForReuse(ctx context.Context, users userdomain.Repository, userID string, limit int) []userdomain.PasswordHistoryEntry {
	if limit <= 0 {
		limit = DefaultPasswordHistoryCount
	}
	history, err := users.GetPasswordHistory(ctx, userID, limit)
	if err != nil {
		passwordHistoryFailOpenTotal.Inc()
		slog.ErrorContext(ctx, "password history lookup failed; skipping reuse check",
			"event", "password_history_fail_open",
			"error", err, "user_id", userID)
		return nil
	}
	return history
}

// checkPasswordHistoryEntries compares a candidate password against
// pre-fetched entries, outside any transaction. Callers on a FORCE RLS store
// must fetch entries inside a tenant-scoped transaction (see changePassword).
func checkPasswordHistoryEntries(history []userdomain.PasswordHistoryEntry, newPassword string, limit int) error {
	if limit <= 0 {
		limit = DefaultPasswordHistoryCount
	}
	for _, entry := range history {
		matched, err := verifyPasswordHistoryHash(newPassword, entry.PasswordHash)
		if err != nil {
			slog.Warn("failed to verify password history entry", "error", err)
			continue
		}
		if matched {
			return passwordHistoryReuseError(limit)
		}
	}
	return nil
}
