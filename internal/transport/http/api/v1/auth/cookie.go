package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

// sessionCookieSecure 报告会话 Cookie 是否启用 Secure 与 __Host- 前缀（COOKIE_SECURE=true）。
//
// 名字与标志必须由同一个开关切换，理由不是对称美观：__Host- 前缀要求 Secure，而带 Secure 的
// Cookie 在 http:// 下会被浏览器直接丢弃（localhost 之外）。此前名字跟着开关走、Secure 却恒为
// true，于是 HTTP 部署（开发/内网演示）里登录接口返回 200 但浏览器从不回传该 Cookie——表现为
// 「登录成功却仍是未登录」，且没有任何服务端日志。CSRF Cookie 早已按同一开关处理
// （platform/middleware.cookieSecure），这里对齐它。
func sessionCookieSecure() bool {
	return config.CookieSecure()
}

// sessionCookieName 是会话 Cookie 的名字：安全模式下用 __Host- 前缀（同时强制
// Secure / Path=/ / 无 Domain / 不经跨站携带），否则退回普通名字以便 HTTP 开发环境可用。
func sessionCookieName() string {
	if sessionCookieSecure() {
		return "__Host-session"
	}
	return "session_id"
}

// SessionCookieName 是当前进程解析出的会话 Cookie 名（启动时求值一次）。
var SessionCookieName = sessionCookieName()

const sessionCookieTTL = 30 * 24 * time.Hour

func sessionContextFromRepository(r *http.Request, handler Handler) (SessionContext, bool) {
	cookie, err := r.Cookie(sessionCookieName())
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return SessionContext{}, false
	}

	resolved, err := handler.ResolveSessionContextForClient(r.Context(), strings.TrimSpace(cookie.Value), util.ClientIP(r), r.UserAgent())
	if err != nil || resolved == nil {
		return SessionContext{}, false
	}

	return *resolved, true
}

func OpaqueSessionIDFromRequest(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName())
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", false
	}

	return strings.TrimSpace(cookie.Value), true
}

// defaultCookieAttrs returns attributes shared by every session cookie.
// __Host- prefix requires Secure + Path=/ + no Domain + SameSite=Strict; Secure 跟随
// COOKIE_SECURE（见 sessionCookieSecure），否则 HTTP 环境下浏览器根本不存这个 Cookie。
//
// SameSite 同步跟随 Secure 开关：Strict 只在 HTTPS + __Host- 前缀下使用（浏览器强制）；
// HTTP 开发环境退回 Lax——允许 fetch/XHR 同源请求携带（Strict 会让 SPA 的 session bootstrap
// 和后续所有 fetch 都"悄悄"不发 cookie，表现为"登录成功但 401 回到登录页"），同时保持
// 比 None 严格的 CSRF 兜底。我们还有 Double Submit Cookie (csrf_token) 作为第二道防线。
func defaultCookieAttrs() http.Cookie {
	sameSite := http.SameSiteLaxMode
	if sessionCookieSecure() {
		sameSite = http.SameSiteStrictMode
	}
	return http.Cookie{
		Path:     "/",
		HttpOnly: true,
		Secure:   sessionCookieSecure(),
		SameSite: sameSite,
	}
}

func ClearSessionCookie(w http.ResponseWriter) {
	c := defaultCookieAttrs()
	// 用运行时求值的名字（与上面的属性同一时刻），而不是启动快照 SessionCookieName：
	// 名字与 Secure 标志必须来自同一个开关，否则又会出现「名字安全模式、标志开发模式」的错位。
	c.Name = sessionCookieName()
	c.Value = ""
	c.Expires = time.Unix(0, 0).UTC()
	c.MaxAge = -1
	http.SetCookie(w, &c)
}

func SetOpaqueSessionCookie(w http.ResponseWriter, sessionID string) {
	c := defaultCookieAttrs()
	c.Name = sessionCookieName()
	c.Value = sessionID
	c.Expires = time.Now().UTC().Add(sessionCookieTTL)
	c.MaxAge = int(sessionCookieTTL / time.Second)
	http.SetCookie(w, &c)
}

// ExtractPreviousSessionIDFromRequest returns the opaque session ID carried by
// the __Host-session cookie, or "" when none is present. Exported so chi.go and
// request.go can both wire PreviousSessionID into login command structs.
func ExtractPreviousSessionIDFromRequest(r *http.Request) string {
	if id, ok := OpaqueSessionIDFromRequest(r); ok {
		return id
	}
	return ""
}

const (
	// trustedDeviceCookiePath 覆盖 /api/v1/auth/login 与 /api/v1/auth/trusted-devices*，
	// 不向 /api/v1/** 全量暴露设备凭据。
	trustedDeviceCookiePath = "/api/v1/auth"
	// trustedDeviceCookieTTL 与 trustedDeviceTrustDuration 同值（30 天）；到期后
	// 服务端判定仍以 trusted_devices.expires_at 为准。
	trustedDeviceCookieTTL = 30 * 24 * time.Hour
)

// trustedDeviceCookieName 是信任设备凭据 Cookie 名字的唯一映射点：
// COOKIE_SECURE=true 时用 __Secure- 前缀（浏览器要求 Secure）；HTTP 开发环境
// 退回无前缀普通名字，避免「前缀 + Secure 但 http://」导致浏览器丢 Cookie。
// 注册（Set）、登录（Read）、测试都只调用本函数 / 本函数的启动快照，
// 不允许在任何位置再写一份名字映射。与会话 Cookie 同一开关策略。
func trustedDeviceCookieName() string {
	if sessionCookieSecure() {
		return "__Secure-trusted_device"
	}
	return "trusted_device"
}

// SetTrustedDeviceCredentialCookie 在注册成功时把客户端生成的明文凭据下发一次到
// HttpOnly Cookie（响应体只回掩码，服务端不落明文）。
//
// 四属性固定：
//   - HttpOnly=true：脚本不可读取（降低 XSS 读取面）；
//   - Secure=config.CookieSecure()：HTTPS/生产为 true，HTTP 开发为 false；
//   - SameSite=Strict（Secure）/ Lax（HTTP 开发）：与会话 Cookie 策略一致。
//     SameSite 与 Secure 不是同一维度：HTTP 下 Strict 仍随同站请求发送，
//     真正被挡住的是跨站形态（如 localhost 与 127.0.0.1 在 schemeful
//     same-site 判定下属于不同站）；
//   - Path=/api/v1/auth：只随认证端点携带，不向全部 API 暴露。
//
// MaxAge=2592000（30 天），Expires=now+30d。
func SetTrustedDeviceCredentialCookie(w http.ResponseWriter, credential string) {
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return
	}
	c := defaultCookieAttrs()
	c.Name = trustedDeviceCookieName()
	c.Path = trustedDeviceCookiePath
	c.Value = credential
	c.Expires = time.Now().UTC().Add(trustedDeviceCookieTTL)
	c.MaxAge = int(trustedDeviceCookieTTL / time.Second)
	http.SetCookie(w, &c)
}

// TrustedDeviceCredentialFromRequest 读取信任设备 Cookie，无 Cookie 或空值时返回 ""。
func TrustedDeviceCredentialFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(trustedDeviceCookieName())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}
