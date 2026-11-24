package auth

import (
	"encoding/json"
	"strconv"
	"strings"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

// SessionPolicyConfig is the effective idle-timeout snapshot exposed to the
// console. The backend remains the authority for expiry; the frontend only
// needs the same minute value to render its warning dialog.
type SessionPolicyConfig struct {
	IdleTimeoutMinutes int `json:"idleTimeoutMinutes"`
}

const defaultIdleTimeoutMinutes = 30

func resolveSessionPolicy(org *orgdomain.Organization) SessionPolicyConfig {
	minutes := defaultIdleTimeoutMinutes
	if org != nil {
		if value, ok := numericMinutes(org.Security.SessionPolicy["idleTimeoutMinutes"]); ok && value > 0 {
			minutes = value
		}
	}
	return SessionPolicyConfig{IdleTimeoutMinutes: minutes}
}

func numericMinutes(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err == nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return 0, false
	}
}
