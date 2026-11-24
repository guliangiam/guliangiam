package config

import (
	"os"
	"strings"
)

// =============================================================================
// P0 批次 7 — 启动/第三方集成
// =============================================================================

func HTTPAddr() string          { return strings.TrimSpace(os.Getenv("HTTP_ADDR")) }
func HTTPPort() string          { return strings.TrimSpace(os.Getenv("HTTP_PORT")) }
func SentryDSN() string         { return os.Getenv("SENTRY_DSN") }
func SentryEnvironment() string { return os.Getenv("SENTRY_ENVIRONMENT") }
func SentryRelease() string     { return os.Getenv("SENTRY_RELEASE") }
func ACMEEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("ACME_ENABLED")), "true")
}
func RADIUSEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("RADIUS_ENABLED")), "true")
}
func RADIUSAddr() string          { return strings.TrimSpace(os.Getenv("RADIUS_ADDR")) }
func RADIUSSecret() string        { return os.Getenv("RADIUS_SECRET") }
func StorageLocalBaseDir() string { return strings.TrimSpace(os.Getenv("STORAGE_LOCAL_BASE_DIR")) }
func MetricsScrapeToken() string  { return os.Getenv("METRICS_SCRAPE_TOKEN") }
func PostgresTestDSN() string     { return os.Getenv("POSTGRES_TEST_DSN") }

// =============================================================================
// P0 批次 8 — Redis 限流 / Session 清理 / Messaging
// =============================================================================

// RateLimitRedisDB 是限流 Redis 的 DB 索引（与 session cache 的 DB 索引独立避免冲突）。
func RateLimitRedisDB() int {
	if v, ok := parseIntKey("RATE_LIMIT_REDIS_DB"); ok {
		return v
	}
	return 1
}

// Session cleanup — 后台 goroutine 周期清理过期 session。
func SessionCleanupIntervalMinutes() int {
	if v, ok := parseIntKey("SESSION_CLEANUP_INTERVAL_MINUTES"); ok {
		return v
	}
	return 60
}
func SessionCleanupRetentionHours() int {
	if v, ok := parseIntKey("SESSION_CLEANUP_RETENTION_HOURS"); ok {
		return v
	}
	return 168 // 7 天
}

// SMTP 邮件发送器 — 全部留空表示不注册该 sender。
func SMTPHost() string { return strings.TrimSpace(os.Getenv("SMTP_HOST")) }
func SMTPPort() int {
	if v, ok := parseIntKey("SMTP_PORT"); ok {
		return v
	}
	return 587
}
func SMTPUsername() string { return os.Getenv("SMTP_USERNAME") }
func SMTPPassword() string { return os.Getenv("SMTP_PASSWORD") }
func SMTPFrom() string     { return strings.TrimSpace(os.Getenv("SMTP_FROM")) }

// SendGrid 邮件发送器。
func SendGridAPIKey() string { return os.Getenv("SENDGRID_API_KEY") }
func SendGridFrom() string   { return strings.TrimSpace(os.Getenv("SENDGRID_FROM")) }

// Resend 邮件发送器。
func ResendAPIKey() string { return os.Getenv("RESEND_API_KEY") }
func ResendFrom() string   { return strings.TrimSpace(os.Getenv("RESEND_FROM")) }

// TLSCertFile/TLSKeyFile enable in-process TLS termination when both are set.
// Production clusters normally terminate TLS at the ingress; this path exists
// for self-hosted/on-prem deployments and for end-to-end mTLS testing.
func TLSCertFile() string { return strings.TrimSpace(os.Getenv("TLS_CERT_FILE")) }
func TLSKeyFile() string  { return strings.TrimSpace(os.Getenv("TLS_KEY_FILE")) }

// TLSClientCAFile optionally enables RFC 8705 client-certificate presentation:
// the CA bundle is loaded into tls.Config.ClientCAs and ClientAuth is set to
// VerifyClientCertIfGiven (browsers without certificates keep working).
func TLSClientCAFile() string { return strings.TrimSpace(os.Getenv("TLS_CLIENT_CA_FILE")) }
