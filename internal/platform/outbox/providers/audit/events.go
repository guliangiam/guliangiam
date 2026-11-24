package audit

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/logging"
)

// ─────────────────────────────────────────────────────────────────────
// 审计事件分类常量（按 OWASP C5 和 ISO 27001 §10.7 分类）
// ─────────────────────────────────────────────────────────────────────
//
// 分类原则：每个常量是一个紧凑、可机读的 dot-separated identifier，
// 便于下游 SIEM / Splunk / ELK 做聚合查询（action=auth.login.success）。

// 认证事件 (auth.*)
const (
	EventAuthLoginSuccess    = "auth.login.success"
	EventAuthLoginFailed     = "auth.login.failed"
	EventAuthLogout          = "auth.logout"
	EventAuthSessionRotate   = "auth.session.rotate"
	EventAuthSessionRevoke   = "auth.session.revoke"
	EventAuthMfaEnable       = "auth.mfa.enable"
	EventAuthMfaDisable      = "auth.mfa.disable"
	EventAuthPasswordChanged = "auth.password.changed"
	EventAuthChallengeIssued = "auth.challenge.issued"
)

// 授权事件 (authz.*)
const (
	EventAuthzAllow              = "authz.allow"
	EventAuthzDeny               = "authz.deny"
	EventAuthzCasbinInitFail     = "authz.casbin.init_fail"
	EventAuthzCasbinEnforceError = "authz.casbin.enforce_error"
)

// 协议事件 (protocol.*) — OAuth2 / OIDC / SAML / CAS
const (
	EventProtocolAuthorizeSuccess   = "protocol.authorize.success"
	EventProtocolAuthorizeDenied    = "protocol.authorize.denied"
	EventProtocolTokenIssued        = "protocol.token.issued"
	EventProtocolTokenRevoked       = "protocol.token.revoked"
	EventProtocolTokenIntrospect    = "protocol.token.introspect"
	EventProtocolCodeExchange       = "protocol.code.exchange"
	EventProtocolSamlAssertion      = "protocol.saml.assertion"
	EventProtocolCasServiceValidate = "protocol.cas.service_validate"
	EventProtocolCasProxyDenied     = "protocol.cas.proxy.denied"
)

// 安全边界拦截 (security.*)
const (
	EventSecurityCSRFViolation   = "security.csrf.violation"
	EventSecurityWAFBlock        = "security.waf.block"
	EventSecurityPanicRecover    = "security.panic.recovered"
	EventSecuritySSRFBlock       = "security.ssrf.block"
	EventSecurityRateLimitDeny   = "security.ratelimit.deny"
	EventSecurityBodyLimitExceed = "security.body.limit_exceed"
)

// 密钥 / 凭证事件 (credential.*)
const (
	EventCredentialCreated = "credential.created" //nolint:gosec // G101: audit event name, not a credential
	EventCredentialRevoked = "credential.revoked" //nolint:gosec // G101: audit event name, not a credential
	EventCredentialRotated = "credential.rotated" //nolint:gosec // G101: audit event name, not a credential
)

// ─────────────────────────────────────────────────────────────────────
// 审计级别 = slog.LevelInfo + 4 = slog.LevelWarn(4)。
// 语义：审计/安全事件与 WARN 同级别，按《开发规范》12.10 的 ERROR/WARN 100%
// 规则不会被采样；审计本体仍以 records 表为准（12.5 红线 2）。
// （历史注释曾声称存在独立的 “audit” 级别，实际并不存在。）
// ─────────────────────────────────────────────────────────────────────

const LevelAudit = slog.LevelInfo + 4

// ─────────────────────────────────────────────────────────────────────
// SlogAudit —— 平台级审计打点 helper。
//
// 设计要点：
//  1. 唯一入口 —— 所有中间件 / app service 都调这个函数打审计
//  2. 自动提取上下文字段 —— request_id / trace_id / actor_id 等
//  3. 依赖 slog.Default() —— bootstrap 已配置 OTel bridge + slog exporter
//  4. Level=Audit(+4) —— 高于 Info，默认不下打到 stdout，去 OTel 管道
//  5. 零依赖 infra/audit —— 避免分层违规；持久化由 chain.go + records repo 负责
// ─────────────────────────────────────────────────────────────────────

// SlogAudit logs an audit event through slog at LevelAudit.
// It auto-injects the audit.category and audit.timestamp fields
// that SIEM pipelines (Splunk, Datadog, OTel) use to filter audit streams.
func SlogAudit(ctx context.Context, category string, args ...any) {
	attrs := make([]any, 0, 2+len(args))
	attrs = append(attrs,
		"audit.category", category,
		"audit.timestamp", time.Now().UTC().Format(time.RFC3339Nano),
	)
	attrs = append(attrs, args...)
	slog.Log(ctx, LevelAudit, "audit event", attrs...)
}

// RequestAuditFields extracts common request-level audit fields from r.
// Intended to be called by middleware audit hooks before they call SlogAudit.
//
// Returns a []any ready to append to SlogAudit:
//
//	SlogAudit(r.Context(), EventSecurityCSRFViolation,
//	    RequestAuditFields(r, actorID, resource)... )
func RequestAuditFields(r *http.Request, actorID, resource string) []any {
	ids := logging.RequestIDsFrom(r.Context())
	if ids.TraceID == "" {
		ids.TraceID = ids.RequestID
	}

	fields := make([]any, 0, 12)
	if ids.RequestID != "" {
		fields = append(fields, "request_id", ids.RequestID)
	}
	if ids.TraceID != "" {
		fields = append(fields, "trace_id", ids.TraceID)
	}
	fields = append(fields,
		"http.method", r.Method,
		"http.path", r.URL.Path,
		"http.host", r.Host,
		"remote.addr", r.RemoteAddr,
		"http.user_agent", truncate(r.UserAgent(), 256),
	)
	if actorID != "" {
		fields = append(fields, "actor.id", actorID)
	}
	if resource != "" {
		fields = append(fields, "audit.resource", resource)
	}
	return fields
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// SanitizeUserAgent strips newlines and control chars that would break
// structured log parsing. Returns "" for empty input.
func SanitizeUserAgent(ua string) string {
	ua = strings.TrimSpace(ua)
	ua = strings.ReplaceAll(ua, "\n", " ")
	ua = strings.ReplaceAll(ua, "\r", " ")
	return ua
}
