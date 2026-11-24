package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// P0 批次 1 — 加密密钥
// =============================================================================

func EncryptionKeyCurrent() string  { return currentKeyProvider.EncryptionKeyCurrent() }
func EncryptionKeyLegacy() string   { return currentKeyProvider.EncryptionKeyLegacy() }
func EncryptionKeyPrevious() string { return currentKeyProvider.EncryptionKeyPrevious() }
func EncryptionKeyCompat() string   { return currentKeyProvider.EncryptionKeyCompat() }
func EncryptionAlgorithm() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("ENCRYPTION_ALGORITHM")))
}
func PasswordHistoryPepper() string { return currentKeyProvider.PasswordHistoryPepper() }

// DefaultClientSecretRotationGrace 是 client secret 轮换的默认宽限期。
// 窗口内旧 secret 仍可验签（判定发生在验签时，不引入定时任务）。
const DefaultClientSecretRotationGrace = 24 * time.Hour

// ClientSecretRotationGrace 返回轮换宽限期。未设置或非法（<=0）时回落到 24h。
func ClientSecretRotationGrace() time.Duration {
	value := strings.TrimSpace(os.Getenv("CLIENT_SECRET_ROTATION_GRACE_HOURS"))
	if value == "" {
		return DefaultClientSecretRotationGrace
	}
	hours, err := strconv.Atoi(value)
	if err != nil || hours <= 0 {
		return DefaultClientSecretRotationGrace
	}
	return time.Duration(hours) * time.Hour
}

// DefaultPasswordHistoryPepper is the built-in development fallback pepper.
// It is identical across every checkout, so production must never use it:
// password-history hashes would then share a public, guessable prefix.
const DefaultPasswordHistoryPepper = "guliangiam:password_history:"

// CryptoProfile 返回加密能力档位（default / gm）。空值按 default 处理；
// 非法值由 ValidateOrPanic 与 crypto.ValidateRuntimeProfile 双重拒绝。
func CryptoProfile() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("CRYPTO_PROFILE")))
}

// GMTLSEnabled 报告部署是否显式启用国密 TLS 能力位。启用只是声明，不等于
// 可用：能力位由 internal/platform/crypto 校验双证书材料后裁决。缺失只降级
// （/healthz=degraded），不阻断 gm 档启动。
func GMTLSEnabled() bool {
	value, ok := parseBoolLiteral(os.Getenv("GM_TLS_ENABLED"))
	return ok && value
}

// 国密 TLS 双证书材料路径。本进程不做 TLCP 握手，只读取这些文件做能力位
// 自检（部署层 TLS 终端负责实际终止）。
func GMTLSSignCertFile() string {
	return strings.TrimSpace(os.Getenv("GM_TLS_SIGN_CERT_FILE"))
}
func GMTLSSignKeyFile() string { return strings.TrimSpace(os.Getenv("GM_TLS_SIGN_KEY_FILE")) }
func GMTLSEncCertFile() string { return strings.TrimSpace(os.Getenv("GM_TLS_ENC_CERT_FILE")) }
func GMTLSEncKeyFile() string  { return strings.TrimSpace(os.Getenv("GM_TLS_ENC_KEY_FILE")) }
func GMTLSCAFile() string      { return strings.TrimSpace(os.Getenv("GM_TLS_CA_FILE")) }

// =============================================================================
// P0 批次 2 — 速率限制 (RateLimit struct + kind 工厂)
// =============================================================================

// RateLimit 描述某端点的令牌桶参数。
type RateLimit struct {
	Rate  float64
	Burst int
}

func RateLimitGlobal() (RateLimit, bool) {
	r, okR := parseFloatKey("RATE_LIMIT_GLOBAL_RATE")
	b, okB := parseIntKey("RATE_LIMIT_GLOBAL_BURST")
	if !okR && !okB {
		return RateLimit{}, false
	}
	return RateLimit{Rate: r, Burst: b}, true
}
func RateLimitAuth() (RateLimit, bool) {
	r, okR := parseFloatKey("RATE_LIMIT_AUTH_RATE")
	b, okB := parseIntKey("RATE_LIMIT_AUTH_BURST")
	if !okR && !okB {
		return RateLimit{}, false
	}
	return RateLimit{Rate: r, Burst: b}, true
}
func RateLimitSignup() (RateLimit, bool) {
	r, okR := parseFloatKey("RATE_LIMIT_SIGNUP_RATE")
	b, okB := parseIntKey("RATE_LIMIT_SIGNUP_BURST")
	if !okR && !okB {
		return RateLimit{}, false
	}
	return RateLimit{Rate: r, Burst: b}, true
}
func RateLimitPasswordReset() (RateLimit, bool) {
	r, okR := parseFloatKey("RATE_LIMIT_PASSWORD_RESET_RATE")
	b, okB := parseIntKey("RATE_LIMIT_PASSWORD_RESET_BURST")
	if !okR && !okB {
		return RateLimit{}, false
	}
	return RateLimit{Rate: r, Burst: b}, true
}
func RateLimitToken() (RateLimit, bool) {
	r, okR := parseFloatKey("RATE_LIMIT_TOKEN_RATE")
	b, okB := parseIntKey("RATE_LIMIT_TOKEN_BURST")
	if !okR && !okB {
		return RateLimit{}, false
	}
	return RateLimit{Rate: r, Burst: b}, true
}
func RateLimitOTLP() (RateLimit, bool) {
	r, okR := parseFloatKey("RATE_LIMIT_OTLP_RATE")
	b, okB := parseIntKey("RATE_LIMIT_OTLP_BURST")
	if !okR && !okB {
		return RateLimit{}, false
	}
	return RateLimit{Rate: r, Burst: b}, true
}

// RateLimitAuditDenialRate 返回拒绝审计令牌桶速率（事件/秒）。
// 未设置或非法（<=0）时回落到 10。
func RateLimitAuditDenialRate() int {
	if n, ok := parseIntKey("RATE_LIMIT_AUDIT_DENIAL_RATE"); ok && n > 0 {
		return n
	}
	return 10
}

func RateLimitRedisAddr() string     { return strings.TrimSpace(os.Getenv("RATE_LIMIT_REDIS_ADDR")) }
func RateLimitRedisPassword() string { return os.Getenv("RATE_LIMIT_REDIS_PASSWORD") }

// PasswordBreachCheckEnabled reports whether candidate passwords are checked
// against the HIBP breached-password corpus (AUTH-009). Default false keeps
// the process offline; enabling it sends only the 5-char SHA-1 prefix
// (k-anonymity) to the range API.
func PasswordBreachCheckEnabled() bool {
	value, ok := parseBoolLiteral(os.Getenv("PASSWORD_BREACH_CHECK_ENABLED"))
	return ok && value
}

// PasswordBreachCheckBaseURL is the range API base; the override exists for
// self-hosted mirrors and isolated test environments.
func PasswordBreachCheckBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("PASSWORD_BREACH_CHECK_BASE_URL")); v != "" {
		return v
	}
	return "https://api.pwnedpasswords.com"
}

// =============================================================================
// P0 批次 3 — Cookie/Session
// =============================================================================

func CookieSecure() bool {
	raw := strings.TrimSpace(os.Getenv("COOKIE_SECURE"))
	if raw == "" {
		return IsProduction()
	}
	if value, ok := parseBoolLiteral(raw); ok {
		return value
	}
	// 非法安全开关不得静默关闭 Secure Cookie。
	return true
}
func CursorTTLSeconds() int {
	if n, ok := parseIntKey("CURSOR_TTL_SECONDS"); ok && n > 0 {
		return n
	}
	return 3600
}
func CursorHMACKey() string { return currentKeyProvider.CursorHMACKey() }
func ETagHMACKey() string   { return currentKeyProvider.ETagHMACKey() }

// =============================================================================
// P0 批次 4 — Security Headers
// =============================================================================

func CSPReportToURL() string { return strings.TrimSpace(os.Getenv("CSP_REPORT_TO_URL")) }

// P0 — CORS
func CORSAllowedOrigins() string { return strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")) }

// SCIMAllowPrivateTargets reports whether SCIM outbound calls may target
// private/loopback addresses. Default false keeps SCIM behind the SSRF-safe
// transport; on-prem connectors can opt in explicitly.
func SCIMAllowPrivateTargets() bool {
	value, ok := parseBoolLiteral(os.Getenv("SCIM_ALLOW_PRIVATE_TARGETS"))
	return ok && value
}

// TrustedProxyCIDRs returns the comma-separated CIDR list of reverse proxies
// whose X-Forwarded-For / X-Real-IP headers may be trusted. Empty means no
// proxy is trusted: forwarded headers are attacker-controlled and ignored.
func TrustedProxyCIDRs() string { return strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS")) }

// MTLSProxyClientCertHeader names the header a trusted reverse proxy uses to
// pass the client certificate chain it already validated. Empty disables
// header trust entirely: forwarded certificate headers are attacker-controlled
// and ignored (RFC 8705 client authentication then needs in-process TLS).
func MTLSProxyClientCertHeader() string {
	return strings.TrimSpace(os.Getenv("MTLS_PROXY_CLIENT_CERT_HEADER"))
}

// ValidHeaderName reports whether name is an RFC 7230 header field name:
// one or more token characters from tchar. It rejects empty names and values
// containing separators, whitespace, or control bytes.
func ValidHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.ContainsRune("!#$%&'*+-.^_`|~", rune(c)):
		default:
			return false
		}
	}
	return true
}
func CORSAllowedMethods() string { return strings.TrimSpace(os.Getenv("CORS_ALLOWED_METHODS")) }
func CORSMaxAgeSeconds() int {
	if n, ok := parseIntKey("CORS_MAX_AGE"); ok && n > 0 {
		return n
	}
	return 600
}

// PasswordChangeEnforcement returns the raw password-change gate mode
// (off|warn|enforce). Normalisation and invalid-value logging happen in the
// transport middleware so this package stays free of log side effects.
func PasswordChangeEnforcement() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("AUTH_PASSWORD_CHANGE_ENFORCEMENT")))
}

// SQLGuardMode returns the raw runtime SQL guard mode (panic|log|off).
// Mode normalisation stays in platform/sqlguard; this package only reads.
func SQLGuardMode() string {
	return strings.ToLower(os.Getenv("GULIANGIAM_SQL_GUARD_MODE"))
}
