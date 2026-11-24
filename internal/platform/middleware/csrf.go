package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// cookieSecure 控制 session 与 CSRF Cookie 的 Secure 标志和 __Host- 前缀。
// production 未显式配置时默认开启，其他环境默认关闭；COOKIE_SECURE 可显式覆盖。
// __Host- Cookie 必须同时启用 Secure 并通过 HTTPS 使用。
func cookieSecure() bool {
	return config.CookieSecure()
}

// csrfCookieName is __Host-csrf in production (Secure, Path=/, HttpOnly=false
// so frontend JS can read it for the Double Submit Cookie pattern), but in
// HTTP dev we drop the __Host- prefix and Secure flag so browsers actually
// store it on non-HTTPS connections.
func csrfCookieName() string {
	if cookieSecure() {
		return "__Host-csrf"
	}
	return "csrf_token"
}

// CSRFTokenMiddleware implements the Double Submit Cookie pattern for CSRF protection.
// On safe methods (GET, HEAD, OPTIONS), it sets a csrf_token cookie if not present.
// On mutating methods (POST, PUT, PATCH, DELETE), it validates the X-CSRF-Token
// header against the csrf_token cookie.
// 仅豁免已被 SessionContextMiddleware 验证的 Bearer/JWT 会话，因为浏览器不会
// 自动附带 Bearer Token。调用方必须显式提供验证器；原始 Authorization 或
// X-Access-Token 请求头本身永远不能绕过 CSRF。
func CSRFTokenMiddleware(bearerValidators ...func(context.Context) bool) func(http.Handler) http.Handler {
	return csrfTokenMiddleware(nil, bearerValidators...)
}

// CSRFTokenMiddlewareWithExemption is CSRFTokenMiddleware plus a per-request
// exemption predicate. It exists for endpoints whose caller cannot hold a CSRF
// token because it never received a session cookie — the client-authenticated
// token endpoints (Basic / client_secret_post / private_key_jwt / mTLS), which
// bootstrap wires to the same exact-path predicate the PEP uses.
//
// The predicate is evaluated before the double-submit check and must be an
// exact match on a closed set: a prefix here would silently exempt every
// future endpoint below it.
func CSRFTokenMiddlewareWithExemption(
	exempt func(*http.Request) bool,
	bearerValidators ...func(context.Context) bool,
) func(http.Handler) http.Handler {
	return csrfTokenMiddleware(exempt, bearerValidators...)
}

func csrfTokenMiddleware(
	exempt func(*http.Request) bool,
	bearerValidators ...func(context.Context) bool,
) func(http.Handler) http.Handler {
	var bearerValidated func(context.Context) bool
	if len(bearerValidators) > 0 {
		bearerValidated = bearerValidators[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				// Safe methods: ensure CSRF cookie exists.
				ensureCSRFCookie(w, r)
				next.ServeHTTP(w, r)
				return
			default:
				if exempt != nil && exempt(r) {
					next.ServeHTTP(w, r)
					return
				}
				if bearerValidated != nil && bearerValidated(r.Context()) {
					next.ServeHTTP(w, r)
					return
				}

				// Cookie-based requests: validate CSRF token.
				if !validateCSRFToken(r) {
					// Security boundary audit: CSRF token violation.
					auditFields := audit.RequestAuditFields(r, "", "csrf")
					auditFields = append(auditFields, "reason", "token_mismatch_or_missing")
					audit.SlogAudit(r.Context(), audit.EventSecurityCSRFViolation, auditFields...)
					httperr.WriteError(w, r, apperrors.NewForbidden("invalid CSRF token"))
					return
				}
				next.ServeHTTP(w, r)
			}
		})
	}
}

// csrfCookieName uses the __Host- prefix (同 __Host-session)：
//   - Domain must be empty（never sent cross-subdomain）
//   - Secure 强制（HTTPS-only）
//   - Path=/ 强制
//
// 与 session cookie（__Host-session, SameSite=Strict, HttpOnly=true）配套：
//
//	session cookie 强 Strict 防止跨站携带，csrf cookie 用 Lax + HttpOnly=false
//	因为 Double Submit Cookie 模式需要前端 JS 读 cookie 再写 header。
//
// ensureCSRFCookie sets a __Host-csrf cookie if one is not already present.
// fail-close 策略：generateCSRFToken 失败时打 slog.Warn 并跳过，
// 客户端没有 cookie → 后续 POST 会 403（安全优于可用性）。
func ensureCSRFCookie(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie(csrfCookieName()); err == nil {
		return // Cookie already exists.
	}

	token, err := generateCSRFToken()
	if err != nil {
		// Fail-close: 生成失败不写 cookie，后续 mutating 请求会 403。
		slog.Warn("failed to generate CSRF token; mutating requests will be rejected", "err", err)
		return
	}

	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Double Submit needs a JS-readable cookie; Secure follows COOKIE_SECURE, SameSite=Lax
		Name:     csrfCookieName(),
		Value:    token,
		Path:     "/",
		HttpOnly: false, // Double Submit Cookie 要求前端 JS 可读
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

// validateCSRFToken checks that the X-CSRF-Token header matches the csrf_token cookie.
func validateCSRFToken(r *http.Request) bool {
	cookie, cookieErr := r.Cookie(csrfCookieName())
	if cookieErr != nil {
		return false
	}

	headerToken := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
	if headerToken == "" {
		return false
	}

	// 定长比较，避免按字节短路比较泄露 token 前缀。
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) == 1
}

// generateCSRFToken generates a cryptographically secure random token.
func generateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
