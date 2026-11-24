package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClearSessionCookie(t *testing.T) {
	// 生产形态（COOKIE_SECURE=true）：Secure + __Host- 前缀 + 立即过期。
	t.Setenv("COOKIE_SECURE", "true")
	w := httptest.NewRecorder()
	ClearSessionCookie(w)

	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, sessionCookieName(), cookies[0].Name)
	assert.Equal(t, "", cookies[0].Value)
	assert.Equal(t, -1, cookies[0].MaxAge)
	assert.True(t, cookies[0].HttpOnly)
	assert.True(t, cookies[0].Secure)
	assert.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
}

func TestSetOpaqueSessionCookie(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "true")
	w := httptest.NewRecorder()
	SetOpaqueSessionCookie(w, "session-id-123")

	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, sessionCookieName(), cookies[0].Name)
	assert.Equal(t, "session-id-123", cookies[0].Value)
	assert.True(t, cookies[0].HttpOnly)
	assert.True(t, cookies[0].Secure)
	assert.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
	assert.Equal(t, "/", cookies[0].Path)
}

func TestOpaqueSessionIDFromRequest_NoCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	_, ok := OpaqueSessionIDFromRequest(r)
	assert.False(t, ok)
}

func TestOpaqueSessionIDFromRequest_ValidOpaque(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "opaque-session-id"})
	id, ok := OpaqueSessionIDFromRequest(r)
	assert.True(t, ok)
	assert.Equal(t, "opaque-session-id", id)
}

func TestSessionCookieName_FollowsCookieSecure(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "true")
	assert.Equal(t, "__Host-session", sessionCookieName(), "__Host- 前缀要求 Secure，安全模式必须带上")

	t.Setenv("COOKIE_SECURE", "false")
	assert.Equal(t, "session_id", sessionCookieName())

	t.Setenv("COOKIE_SECURE", "")
	assert.Equal(t, "session_id", sessionCookieName())
}

func TestTrustedDeviceCookieName_FollowsCookieSecure(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "true")
	assert.Equal(t, "__Secure-trusted_device", trustedDeviceCookieName())

	t.Setenv("COOKIE_SECURE", "false")
	assert.Equal(t, "trusted_device", trustedDeviceCookieName())

	t.Setenv("COOKIE_SECURE", "")
	assert.Equal(t, "trusted_device", trustedDeviceCookieName())
}

func TestSetTrustedDeviceCredentialCookie_SecureMode(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "true")
	w := httptest.NewRecorder()
	SetTrustedDeviceCredentialCookie(w, "  client-credential  ")

	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1)
	c := cookies[0]
	assert.Equal(t, "__Secure-trusted_device", c.Name)
	assert.Equal(t, "client-credential", c.Value)
	assert.Equal(t, "/api/v1/auth", c.Path)
	assert.True(t, c.HttpOnly)
	assert.True(t, c.Secure)
	assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
	assert.Equal(t, 2592000, c.MaxAge)
}

func TestSetTrustedDeviceCredentialCookie_DevMode(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "false")
	w := httptest.NewRecorder()
	SetTrustedDeviceCredentialCookie(w, "client-credential")

	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1)
	c := cookies[0]
	assert.Equal(t, "trusted_device", c.Name)
	assert.Equal(t, "client-credential", c.Value)
	assert.Equal(t, "/api/v1/auth", c.Path)
	assert.True(t, c.HttpOnly, "HttpOnly 两种模式都必须为 true")
	assert.False(t, c.Secure, "HTTP 开发环境不能带 Secure，否则浏览器丢弃")
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Equal(t, 2592000, c.MaxAge)
}

func TestSetTrustedDeviceCredentialCookie_EmptyCredentialIsNoop(t *testing.T) {
	w := httptest.NewRecorder()
	SetTrustedDeviceCredentialCookie(w, "   ")
	assert.Empty(t, w.Result().Cookies())
}

func TestTrustedDeviceCredentialFromRequest(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "false")
	r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	assert.Equal(t, "", TrustedDeviceCredentialFromRequest(r))

	r.AddCookie(&http.Cookie{Name: "trusted_device", Value: "  from-cookie  "})
	assert.Equal(t, "from-cookie", TrustedDeviceCredentialFromRequest(r))
}

func TestDefaultCookieAttrs_SecureFollowsCookieSecure(t *testing.T) {
	// 名字与 Secure 标志必须同一个开关：Secure Cookie 在 http:// 下会被浏览器丢弃，
	// 而 __Host- 前缀又要求 Secure，二者错位就会出现「登录成功却仍匿名」。
	//
	// SameSite 也同步跟随：Secure 模式用 Strict（__Host- 语义 + HTTPS 生产环境）；
	// HTTP 开发模式退回 Lax——让 SPA fetch/XHR 能带上 session cookie（Strict 会让浏览器
	// 对 fetch 静默丢弃 cookie），同时保留 CSRF 兜底。
	t.Setenv("COOKIE_SECURE", "true")
	secure := defaultCookieAttrs()
	assert.True(t, secure.Secure)
	assert.True(t, secure.HttpOnly)
	assert.Equal(t, "/", secure.Path)
	assert.Equal(t, http.SameSiteStrictMode, secure.SameSite, "Secure/HTTPS 必须 Strict（__Host- 强制）")
	assert.Empty(t, secure.Domain, "__Host- 要求不带 Domain")

	t.Setenv("COOKIE_SECURE", "")
	dev := defaultCookieAttrs()
	assert.False(t, dev.Secure, "HTTP 开发环境不能带 Secure，否则浏览器不存 Cookie")
	assert.True(t, dev.HttpOnly, "HttpOnly 与安全模式无关，始终开启")
	assert.Equal(t, http.SameSiteLaxMode, dev.SameSite, "HTTP/SPA 开发必须 Lax 才能让 fetch 携带 cookie")
}

func TestSetOpaqueSessionCookie_AttributesPerMode(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "true")
	rec := httptest.NewRecorder()
	SetOpaqueSessionCookie(rec, "opaque-session-id")
	cookies := rec.Result().Cookies()
	if assert.Len(t, cookies, 1) {
		assert.Equal(t, sessionCookieName(), cookies[0].Name)
		assert.True(t, cookies[0].Secure)
		assert.True(t, cookies[0].HttpOnly)
		assert.Equal(t, "/", cookies[0].Path)
	}

	t.Setenv("COOKIE_SECURE", "")
	rec = httptest.NewRecorder()
	SetOpaqueSessionCookie(rec, "opaque-session-id")
	cookies = rec.Result().Cookies()
	if assert.Len(t, cookies, 1) {
		assert.Equal(t, sessionCookieName(), cookies[0].Name)
		assert.False(t, cookies[0].Secure)
		assert.Equal(t, "opaque-session-id", cookies[0].Value)
	}
}
