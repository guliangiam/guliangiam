package organizations

import (
	"strconv"
	"strings"
)

// 密码策略的默认值全后端唯一来源。组织未配置策略时，认证/用户管理/改密
// 各条链路都必须通过 EffectivePasswordPolicy 取到同一组默认值，禁止在
// app 层再声明一份默认常量或硬编码长度。
const (
	// 与 requirements/non-functional.md「密码最低长度 ≥ 14 位」一致；
	// 组织只能加严，不能低于该下限（DEF-11）。
	DefaultPasswordMinLength = 14
	DefaultPasswordMaxLength = 128
)

// PasswordPolicy 是组织密码策略的生效结果：组织配置叠加默认值后的快照。
// 该结构同时用于后端校验和账户上下文的 JSON 输出，保证前端展示与后端
// 判定使用同一份数据。
type PasswordPolicy struct {
	MinLength             int  `json:"minLength"`
	MaxLength             int  `json:"maxLength"`
	RequireUppercase      bool `json:"requireUppercase"`
	RequireLowercase      bool `json:"requireLowercase"`
	RequireDigit          bool `json:"requireDigit"`
	RequireSpecial        bool `json:"requireSpecial"`
	RejectCommonPasswords bool `json:"rejectCommonPasswords"`
	RejectUsername        bool `json:"rejectUsername"`
	PasswordHistoryCount  int  `json:"passwordHistoryCount"` // Number of previous passwords to check (0 = disabled)
	PasswordMaxAgeDays    int  `json:"passwordMaxAgeDays"`   // Days before password expires (0 = never expires)
}

// DefaultPasswordPolicy 返回未配置组织策略时的默认密码策略。
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:             DefaultPasswordMinLength,
		MaxLength:             DefaultPasswordMaxLength,
		RequireUppercase:      true,
		RequireLowercase:      true,
		RequireDigit:          true,
		RequireSpecial:        true,
		RejectCommonPasswords: true,
		RejectUsername:        true,
		PasswordHistoryCount:  5,
		PasswordMaxAgeDays:    90,
	}
}

// EffectivePasswordPolicy 将组织的 security.passwordPolicy 叠加到默认值上。
// 组织或策略为空时返回默认策略，调用方无需再各自处理缺省逻辑。
func EffectivePasswordPolicy(org *Organization) PasswordPolicy {
	policy := DefaultPasswordPolicy()
	if org == nil || len(org.Security.PasswordPolicy) == 0 {
		return policy
	}

	pp := org.Security.PasswordPolicy
	if v, ok := passwordPolicyInt(pp, "minLength"); ok {
		policy.MinLength = v
	}
	if v, ok := passwordPolicyBool(pp, "requireUppercase"); ok {
		policy.RequireUppercase = v
	}
	if v, ok := passwordPolicyBool(pp, "requireLowercase"); ok {
		policy.RequireLowercase = v
	}
	if v, ok := passwordPolicyBool(pp, "requireDigit"); ok {
		policy.RequireDigit = v
	}
	if v, ok := passwordPolicyBool(pp, "requireSpecial"); ok {
		policy.RequireSpecial = v
	}
	if v, ok := passwordPolicyBool(pp, "rejectCommonPasswords"); ok {
		policy.RejectCommonPasswords = v
	}
	if v, ok := passwordPolicyBool(pp, "rejectUsername"); ok {
		policy.RejectUsername = v
	}
	if v, ok := passwordPolicyInt(pp, "maxLength"); ok {
		policy.MaxLength = v
	}
	if v, ok := passwordPolicyInt(pp, "passwordHistoryCount"); ok {
		policy.PasswordHistoryCount = v
	}
	if v, ok := passwordPolicyInt(pp, "passwordMaxAgeDays"); ok {
		policy.PasswordMaxAgeDays = v
	}

	return policy
}

func passwordPolicyBool(m map[string]any, key string) (value, found bool) {
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

func passwordPolicyInt(m map[string]any, key string) (int, bool) {
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
