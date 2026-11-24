package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// PasswordPolicyConfig 是 domain 层生效策略的别名，保留既有调用点与测试。
type PasswordPolicyConfig = orgdomain.PasswordPolicy

// defaultPasswordPolicy 返回全后端唯一的默认密码策略。
func defaultPasswordPolicy() PasswordPolicyConfig {
	return orgdomain.DefaultPasswordPolicy()
}

// resolvePasswordPolicy 将组织策略叠加到默认值；缺省逻辑统一由 domain 负责。
func resolvePasswordPolicy(org *orgdomain.Organization) PasswordPolicyConfig {
	return orgdomain.EffectivePasswordPolicy(org)
}

// ValidatePasswordPolicy validates the given password against the organization's password policy.
// The username parameter is used for the reject-username check.
func (s *Service) ValidatePasswordPolicy(ctx context.Context, orgID, username, password string) error {
	org, err := s.UOW.Organizations().GetByID(ctx, orgID)
	if err != nil {
		return err
	}

	policy := resolvePasswordPolicy(org)
	return validatePasswordAgainstPolicy(password, username, policy)
}

// ValidatePasswordPolicyWithHistory validates password policy and checks against password history.
// Must be called within a transaction context where user credentials are accessible.
func (s *Service) ValidatePasswordPolicyWithHistory(ctx context.Context, tx appuow.UnitOfWork, orgID, userID, username, password string) error {
	org, err := tx.Organizations().GetByID(ctx, orgID)
	if err != nil {
		return err
	}

	policy := resolvePasswordPolicy(org)

	if err := validatePasswordAgainstPolicy(password, username, policy); err != nil {
		return err
	}

	// Check password history
	if policy.PasswordHistoryCount > 0 {
		if err := checkPasswordHistory(ctx, tx, userID, password, policy.PasswordHistoryCount); err != nil {
			return err
		}
	}

	return nil
}

// checkPasswordHistory verifies the new password doesn't match any of the last N passwords.
func checkPasswordHistory(ctx context.Context, tx appuow.UnitOfWork, userID, newPassword string, historyCount int) error {
	history, err := tx.Users().GetPasswordHistory(ctx, userID, historyCount)
	if err != nil {
		return apperrors.NewInternal("retrieve password history")
	}

	for i := range history {
		matched, err := verifyPasswordHistoryHash(newPassword, history[i].PasswordHash)
		if err != nil {
			slog.WarnContext(ctx, "failed to verify password history entry", "error", err)
			continue
		}
		if matched {
			return passwordHistoryReuseError(historyCount)
		}
	}

	return nil
}

func validatePasswordAgainstPolicy(password, username string, policy PasswordPolicyConfig) error {
	passwordLength := utf8.RuneCountInString(password)
	if passwordLength < policy.MinLength {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("password must be at least %d characters long", policy.MinLength), map[string]any{"field": "password", "min": policy.MinLength})
	}

	if policy.MaxLength > 0 && passwordLength > policy.MaxLength {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("password must not exceed %d characters", policy.MaxLength), map[string]any{"field": "password", "max": policy.MaxLength})
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
		return apperrors.NewUnprocessableEntity("password must contain at least one uppercase letter")
	}
	if policy.RequireLowercase && !hasLower {
		return apperrors.NewUnprocessableEntity("password must contain at least one lowercase letter")
	}
	if policy.RequireDigit && !hasDigit {
		return apperrors.NewUnprocessableEntity("password must contain at least one digit")
	}
	if policy.RequireSpecial && !hasSpecial {
		return apperrors.NewUnprocessableEntity("password must contain at least one special character")
	}

	if policy.RejectUsername && username != "" {
		if strings.EqualFold(password, username) || strings.Contains(strings.ToLower(password), strings.ToLower(username)) {
			return apperrors.NewUnprocessableEntity("password must not contain the username")
		}
	}

	if policy.RejectCommonPasswords && isCommonPassword(password) {
		return apperrors.NewUnprocessableEntity("password is too common, please choose a stronger password")
	}

	if policy.MinLength >= 12 && calculatePasswordComplexityScore(password) < 3 {
		return apperrors.NewUnprocessableEntity("password does not meet complexity requirements; use a mix of character types")
	}

	if hasSequentialChars(password, 4) {
		return apperrors.NewUnprocessableEntity("password must not contain 4 or more sequential characters")
	}

	if hasRepeatedChars(password, 4) {
		return apperrors.NewUnprocessableEntity("password must not contain 4 or more repeated characters")
	}

	return nil
}

// isCommonPassword checks if the password is in the top common passwords list.
func calculatePasswordComplexityScore(password string) int {
	score := 0
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
	if hasUpper {
		score++
	}
	if hasLower {
		score++
	}
	if hasDigit {
		score++
	}
	if hasSpecial {
		score++
	}
	return score
}

func hasSequentialChars(password string, length int) bool {
	if len(password) < length {
		return false
	}
	runes := []rune(password)
	count := 1
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1]+1 || runes[i] == runes[i-1]-1 {
			count++
			if count >= length {
				return true
			}
		} else {
			count = 1
		}
	}
	return false
}

func hasRepeatedChars(password string, length int) bool {
	if len(password) < length {
		return false
	}
	runes := []rune(password)
	count := 1
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			count++
			if count >= length {
				return true
			}
		} else {
			count = 1
		}
	}
	return false
}

func isCommonPassword(password string) bool {
	lower := strings.ToLower(password)
	_, found := commonPasswords[lower]
	return found
}

// boolFromMap extracts a bool value from a map[string]any.
func boolFromMap(m map[string]any, key string) (value, found bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[key]
	if !ok {
		return false, false
	}
	switch typed := v.(type) {
	case bool:
		return typed, true
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true"), true
	default:
		return false, false
	}
}

// intFromMap extracts an int value from a map[string]any.
func intFromMap(m map[string]any, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch typed := v.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

// commonPasswords is a set of the most commonly used passwords.
// This is a subset of the top 1000 most common passwords.
var commonPasswords = map[string]struct{}{
	"123456":            {},
	loginMethodPassword: {},
	"12345678":          {},
	"qwerty":            {},
	"123456789":         {},
	"12345":             {},
	"1234":              {},
	"111111":            {},
	"1234567":           {},
	"dragon":            {},
	"123123":            {},
	"baseball":          {},
	"abc123":            {},
	"football":          {},
	"monkey":            {},
	"letmein":           {},
	"shadow":            {},
	"master":            {},
	"696969":            {},
	"michael":           {},
	"mustang":           {},
	"666666":            {},
	"access":            {},
	"123321":            {},
	"654321":            {},
	"superman":          {},
	"1qaz2wsx":          {},
	"7777777":           {},
	"fuckyou":           {},
	"121212":            {},
	"000000":            {},
	"qazwsx":            {},
	"123qwe":            {},
	"killer":            {},
	"trustno1":          {},
	"jordan":            {},
	"jennifer":          {},
	"zxcvbnm":           {},
	"asdfgh":            {},
	"hunter":            {},
	"buster":            {},
	"soccer":            {},
	"harley":            {},
	"batman":            {},
	"andrew":            {},
	"tigger":            {},
	"sunshine":          {},
	"iloveyou":          {},
	"2000":              {},
	"charlie":           {},
	"robert":            {},
	"thomas":            {},
	"hockey":            {},
	"ranger":            {},
	"daniel":            {},
	"starwars":          {},
	"klaster":           {},
	"112233":            {},
	"george":            {},
	"computer":          {},
	"michelle":          {},
	"jessica":           {},
	"pepper":            {},
	"1111":              {},
	"zxcvbn":            {},
	"555555":            {},
	"11111111":          {},
	"131313":            {},
	"freedom":           {},
	"777777":            {},
	"pass":              {},
	"fuck":              {},
	"maggie":            {},
	"159753":            {},
	"aaaaaa":            {},
	"ginger":            {},
	"princess":          {},
	"joshua":            {},
	"cheese":            {},
	"amanda":            {},
	"summer":            {},
	"love":              {},
	"ashley":            {},
	"6969":              {},
	"nicole":            {},
	"chelsea":           {},
	"biteme":            {},
	"matthew":           {},
	"access14":          {},
	"yankees":           {},
	"987654321":         {},
	"dallas":            {},
	"austin":            {},
	"thunder":           {},
	"taylor":            {},
	"matrix":            {},
	"william":           {},
	"corvette":          {},
	"heaven":            {},
	"martin":            {},
	"heather":           {},
	"secret":            {},
	"fucker":            {},
	"merlin":            {},
	"goldfish":          {},
	"2112":              {},
	"test":              {},
	"admin":             {},
	"welcome":           {},
	"login":             {},
	"guest":             {},
	"password1":         {},
	"p@ssw0rd":          {},
	"p@ssword":          {},
	"passw0rd":          {},
	"letmein1":          {},
	"changeme":          {},
	"qwerty123":         {},
	"1q2w3e4r":          {},
	"q1w2e3r4":          {},
	"123abc":            {},
	"iloveu":            {},
	"password123":       {},
}
