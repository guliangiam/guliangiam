package users

import (
	"fmt"
	"strings"
	"unicode"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/passwordhistory"
)

// validatePasswordComplexity checks a password against the organization's
// effective policy. The policy must come from
// organizations.EffectivePasswordPolicy so admin-set passwords and
// self-service changes share one threshold; there is no local default here.
func validatePasswordComplexity(password, username string, policy orgdomain.PasswordPolicy) error {
	if len(password) < policy.MinLength {
		return passwordRuleError(fmt.Sprintf("password must be at least %d characters long", policy.MinLength), map[string]any{metaKeyField: "password", "min": policy.MinLength})
	}
	if policy.MaxLength > 0 && len(password) > policy.MaxLength {
		return passwordRuleError(fmt.Sprintf("password must not exceed %d characters", policy.MaxLength), map[string]any{metaKeyField: "password", "max": policy.MaxLength})
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if policy.RequireUppercase && !hasUpper {
		return passwordRuleError("password must contain at least one uppercase letter", nil)
	}
	if policy.RequireLowercase && !hasLower {
		return passwordRuleError("password must contain at least one lowercase letter", nil)
	}
	if policy.RequireDigit && !hasDigit {
		return passwordRuleError("password must contain at least one digit", nil)
	}
	if policy.RequireSpecial && !hasSpecial {
		return passwordRuleError("password must contain at least one special character", nil)
	}
	if policy.RejectUsername && username != "" {
		if strings.EqualFold(password, username) || strings.Contains(strings.ToLower(password), strings.ToLower(username)) {
			return passwordRuleError("password must not contain the username", nil)
		}
	}

	return nil
}

// passwordRuleError answers 422: the request bound, but a policy rule rejected
// the value, so the caller succeeds by supplying a different password. The
// password itself is never copied into meta — it is a secret.
func passwordRuleError(message string, extra map[string]any) error {
	meta := map[string]any{metaKeyField: "password"}
	for key, value := range extra {
		meta[key] = value
	}
	return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, message, meta)
}

// checkPasswordHistoryAgainstEntries verifies the new password doesn't match
// any history entry. History rows are hashed by passwordhistory.Hash, so the
// comparison must use the same algorithm; the returned 422 carries a failures
// list so clients can render every failed rule.
func checkPasswordHistoryAgainstEntries(entries []userdomain.PasswordHistoryEntry, newPassword string, historyCount int) error {
	for i := range entries {
		matched, err := passwordhistory.Verify(newPassword, entries[i].PasswordHash)
		if err != nil {
			continue
		}
		if matched {
			message := fmt.Sprintf("password cannot be reused (checked against last %d passwords)", historyCount)
			return passwordRuleError(message, map[string]any{
				"rule":         "password_history",
				"historyCount": historyCount,
				"failures": []map[string]any{{
					metaKeyField: "password",
					"rule":       "password_history",
					"message":    message,
				}},
			})
		}
	}
	return nil
}
