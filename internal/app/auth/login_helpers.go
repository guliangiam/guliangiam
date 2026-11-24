package auth

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/guliangiam/guliangiam/cred"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringConfigValue(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	value, ok := config[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func intConfigValue(config map[string]any, key string) int {
	if config == nil {
		return 0
	}
	value, ok := config[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func hostFromProviderURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		if host, _, err := net.SplitHostPort(trimmed); err == nil {
			return host
		}
		return trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func portFromProviderURL(raw string) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	if !strings.Contains(trimmed, "://") {
		_, port, err := net.SplitHostPort(trimmed)
		if err != nil {
			return 0
		}
		parsed, err := strconv.Atoi(port)
		if err != nil {
			return 0
		}
		return parsed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return 0
	}
	if parsed.Port() == "" {
		return 0
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return 0
	}
	return port
}

// verifyPassword is the single local-password gate. A credential row only
// proves a local password when PasswordHash is non-empty; the algorithm column
// selects a verifier but can never make an empty hash or an external-source
// marker (ldap/plain/unknown) pass. Legacy verifiers are accepted read-only so
// an existing hash can be upgraded to argon2id after successful verification
// (see upgradePasswordAlgorithm).
func verifyPassword(password, algorithm, passwordHash string) error {
	credentials := userdomain.UserCredentials{
		PasswordAlgo: algorithm,
		PasswordHash: passwordHash,
	}
	passwordType, ok := credentials.PasswordVerificationAlgorithm()
	if !ok {
		return apperrors.NewUnauthorized("password sign in is unavailable")
	}

	manager := cred.GetCredManager(passwordType)
	if manager == nil {
		return apperrors.NewUnauthorized("password sign in is unavailable")
	}

	if !manager.IsPasswordCorrect(password, passwordHash, "") {
		return apperrors.NewUnauthorized("invalid username or password")
	}

	return nil
}

func isUserAllowedToLogin(status string, isForbidden bool) bool {
	if isForbidden {
		return false
	}

	status = strings.TrimSpace(status)
	return status == "" || strings.EqualFold(status, statusActive)
}
