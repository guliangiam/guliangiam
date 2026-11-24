package organizations

// 密码派生算法只作用于一次性口令/密钥派生摘要，不作用于密码存储哈希。
// 存储哈希固定 argon2id；SM3 仅作为派生口径的可选实现，避免把国密选项
// 误解为「可以用更弱的密码存储算法」。
const (
	PasswordDeriveAlgorithmHMACSHA256 = "hmac_sha256"
	PasswordDeriveAlgorithmHMACSM3    = "hmac_sm3"
)

type OrganizationSecurityPolicy struct {
	OrganizationID          string
	PasswordPolicy          map[string]any
	PasswordDeriveAlgorithm string
	MfaPolicy               map[string]any
	IPPolicy                map[string]any
	SessionPolicy           map[string]any
	RiskPolicy              map[string]any
	LockoutPolicy           map[string]any
	ConcurrentSessionPolicy map[string]any
	HasMasterKey            bool
}

// EffectivePasswordDeriveAlgorithm 返回落库口径的派生算法；空值按默认
// hmac_sha256 处理，保证未显式设置策略的调用方不会写入空串触发 CHECK。
func (p OrganizationSecurityPolicy) EffectivePasswordDeriveAlgorithm() string {
	if p.PasswordDeriveAlgorithm == "" {
		return PasswordDeriveAlgorithmHMACSHA256
	}
	return p.PasswordDeriveAlgorithm
}
