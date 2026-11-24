package config

import (
	"os"
	"strconv"
	"strings"
)

// IsProduction reports whether the process runs in a production environment.
//
// ADR-021 makes APP_ENV the single source of truth. IS_PRODUCTION is kept only
// as a read-only synonym during the transition and is validated for agreement
// by ValidateOrPanic.
func IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

// IsDevelopment reports whether the process runs in a development environment.
// APP_ENV is the single source of truth (ADR-021 decision 1).
func IsDevelopment() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "development")
}

// DriftImplicitlyAllowed 报告 checksum 漂移是否应该被隐式允许。
// 黑名单语义：只有 production 禁掉隐式漂移；development / test / staging / 空值都允许。
// 生产环境漂移需要显式 ALLOW_CHECKSUM_DRIFT=true，且会触发 warning 日志。
func DriftImplicitlyAllowed() bool {
	return !IsProduction()
}

// parseBoolLiteral 解析布尔字面量（true/false/1/0，大小写不敏感）。第二个返回值
// 为 false 表示「写了但无法解析」——调用方必须区分它与「未设置」：未设置用默认值，
// 写错则在启动校验里直接失败。一个拼错的开关等于静默关掉一条策略，这类错误必须在
// 启动时暴露，而不是等到某次登录才发现策略没生效。
func parseBoolLiteral(raw string) (value, ok bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	default:
		return false, false
	}
}

func parseFloatKey(key string) (float64, bool) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseIntKey(key string) (int, bool) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}
