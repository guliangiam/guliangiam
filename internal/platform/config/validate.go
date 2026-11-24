package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

func ValidateOrPanic() {
	var errors []string

	if strings.TrimSpace(os.Getenv("DB_DSN")) == "" &&
		strings.TrimSpace(os.Getenv("POSTGRES_DSN")) == "" &&
		strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		errors = append(errors, "DB_DSN (or POSTGRES_DSN / DATABASE_URL) is required")
	}

	isProduction := IsProduction()

	// ADR-021 decision 1: IS_PRODUCTION is a read-only synonym of APP_ENV and
	// must never contradict it.
	if raw := strings.TrimSpace(os.Getenv("IS_PRODUCTION")); raw != "" {
		legacy := strings.EqualFold(raw, "true") || raw == "1"
		if legacy != isProduction {
			errors = append(errors, fmt.Sprintf(
				"IS_PRODUCTION=%q contradicts APP_ENV=%q; IS_PRODUCTION is a deprecated synonym of APP_ENV and must agree with it",
				raw, os.Getenv("APP_ENV")))
		}
	}

	// 拼错的布尔开关必须在启动时失败：否则运维以为策略开着，实际是关的。
	if raw := strings.TrimSpace(os.Getenv("WEBAUTHN_REQUIRE_UV")); raw != "" {
		if _, ok := parseBoolLiteral(raw); !ok {
			errors = append(errors, fmt.Sprintf("WEBAUTHN_REQUIRE_UV must be true/false (or 1/0), got %q", raw))
		}
	}
	if raw := strings.TrimSpace(os.Getenv("PASSWORD_BREACH_CHECK_ENABLED")); raw != "" {
		if _, ok := parseBoolLiteral(raw); !ok {
			errors = append(errors, fmt.Sprintf("PASSWORD_BREACH_CHECK_ENABLED must be true/false (or 1/0), got %q", raw))
		}
	}

	// CRYPTO_PROFILE 的取值是能力裁决入口：未知值不得回落到 default，
	// 否则拼错 gm 会静默使用国际算法。具体能力缺失由 crypto 包探测后拒绝。
	switch profile := strings.ToLower(strings.TrimSpace(os.Getenv("CRYPTO_PROFILE"))); profile {
	case "", "default", "gm":
	default:
		errors = append(errors, fmt.Sprintf("CRYPTO_PROFILE must be default or gm, got %q", profile))
	}
	if raw := strings.TrimSpace(os.Getenv("GM_TLS_ENABLED")); raw != "" {
		if _, ok := parseBoolLiteral(raw); !ok {
			errors = append(errors, fmt.Sprintf("GM_TLS_ENABLED must be true/false (or 1/0), got %q", raw))
		}
	}

	// ADR-021 decision 4 / DB-058: confidential columns must never fall back to
	// plaintext, so the field encryption key is required in every environment.
	// Presence is resolved through the selected KeyProvider (env by default,
	// Vault when KMS_PROVIDER=vault), so a KMS-hosted KEK is not misreported as a
	// missing environment variable. Content validation (base64, decoded length,
	// algorithm, SM4 availability) lives in crypto.NewFieldEncryptorChecked, which
	// bootstrap runs before creating any listener: one rule set, not two.
	if strings.TrimSpace(EncryptionKeyCurrent()) == "" && strings.TrimSpace(EncryptionKeyLegacy()) == "" {
		errors = append(errors, "ENCRYPTION_KEY_CURRENT (or ENCRYPTION_KEY) is required in every environment; confidential fields must not fall back to plaintext")
	}

	// Password-history pepper: production must not use the shared development
	// sentinel. Per-organization password-history policies can be enabled at
	// runtime, so startup cannot prove the feature is off; production always
	// requires a deployment-specific pepper.
	if issue := PasswordHistoryPepperIssue(); issue != "" {
		errors = append(errors, issue)
	}

	if isProduction && strings.TrimSpace(os.Getenv("ISSUER_URL")) == "" {
		errors = append(errors, "ISSUER_URL is required in production")
	}

	// SEC-A4: /metrics and /debug/pprof fail open when the token is empty, which
	// would expose heap/goroutine profiles in production. Refuse to start rather
	// than silently serving them unauthenticated.
	if isProduction && strings.TrimSpace(os.Getenv("METRICS_SCRAPE_TOKEN")) == "" {
		errors = append(errors, "METRICS_SCRAPE_TOKEN is required in production; ops endpoints must not be exposed without a token")
	}

	if portStr := strings.TrimSpace(os.Getenv("HTTP_PORT")); portStr != "" {
		if n, err := strconv.Atoi(portStr); err != nil || n <= 0 {
			errors = append(errors, fmt.Sprintf("HTTP_PORT must be a positive integer, got %q", portStr))
		}
	}

	if v := os.Getenv("POSTGRES_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n <= 0 {
			errors = append(errors, fmt.Sprintf("POSTGRES_MAX_CONNS must be a positive integer, got %q", v))
		}
	}

	if v := os.Getenv("RATE_LIMIT_RPM"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n <= 0 {
			errors = append(errors, fmt.Sprintf("RATE_LIMIT_RPM must be a positive integer, got %q", v))
		}
	}

	if len(errors) > 0 {
		for _, e := range errors {
			slog.Error("config validation failed", "error", e)
		}
		panic(fmt.Sprintf("config validation failed: %d error(s)", len(errors)))
	}
}

// PasswordHistoryPepperIssue returns a non-empty message when the effective
// password-history pepper is unusable in production, or "" when it is
// acceptable. The message never contains the pepper value itself.
//
// Non-production keeps the historical fallback (warn on first use, do not
// block startup) so local development without PASSWORD_HISTORY_PEPPER still
// works.
func PasswordHistoryPepperIssue() string {
	if !IsProduction() {
		return ""
	}
	pepper := strings.TrimSpace(PasswordHistoryPepper())
	if pepper == "" {
		return "PASSWORD_HISTORY_PEPPER is required in production; password history must not use the built-in development pepper"
	}
	if pepper == DefaultPasswordHistoryPepper {
		return "PASSWORD_HISTORY_PEPPER must not be the built-in development pepper in production"
	}
	return ""
}
