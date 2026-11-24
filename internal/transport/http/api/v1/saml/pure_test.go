package saml

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
)

func TestUrlEncode(t *testing.T) {
	assert.Equal(t, "hello%2Bworld", urlEncode("hello+world"))
	assert.Equal(t, "noplus", urlEncode("noplus"))
	assert.Equal(t, "", urlEncode(""))
}

func TestEscapeHTML(t *testing.T) {
	assert.Equal(t, "&amp;", escapeHTML("&"))
	assert.Equal(t, "&lt;", escapeHTML("<"))
	assert.Equal(t, "&gt;", escapeHTML(">"))
	assert.Equal(t, "&quot;", escapeHTML("\""))
	assert.Equal(t, "&#39;", escapeHTML("'"))
	assert.Equal(t, "hello", escapeHTML("hello"))
	assert.Equal(t, "&amp;&lt;&gt;", escapeHTML("&<>"))
}

// Fail-close regression guard（PEP fail-close 审计 2026-09-24）：/api/v1/saml/*
// 在 PEP 白名单内，所以中间件不会替它挡下未认证请求。若身份还能从请求头取，任何
// 人带上这三个头就能冒用任意用户取得签名 SAML 断言 —— 认证绕过。伪造的请求头必须
// 被完全忽略，只有 SessionContextMiddleware 注入的 session context 才算数。
func TestSessionFromRequest_IgnoresSpoofableIdentityHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Organization-Slug", "org1")
	r.Header.Set("X-Username", "victim")
	r.Header.Set("X-Access-Token", "token123")

	session, ok := sessionFromRequest(r)
	assert.False(t, ok)
	assert.Equal(t, "", session.Username)
	assert.Equal(t, "", session.OrganizationSlug)
}

func TestSessionFromRequest_Empty(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	session, ok := sessionFromRequest(r)
	assert.False(t, ok)
	assert.Equal(t, "", session.Username)
}

func TestSessionFromRequest_ReadsSessionContext(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(authtransport.WithSessionContext(r.Context(), authtransport.SessionContext{
		OrganizationSlug: "org1",
		Username:         "user1",
		AccessToken:      "token123",
	}))

	session, ok := sessionFromRequest(r)
	assert.True(t, ok)
	assert.Equal(t, "org1", session.OrganizationSlug)
	assert.Equal(t, "user1", session.Username)
	assert.Equal(t, "token123", session.AccessToken)
}

func TestSessionFromRequest_EmptySessionContextIsNotAuthenticated(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(authtransport.WithSessionContext(r.Context(), authtransport.SessionContext{}))

	_, ok := sessionFromRequest(r)
	assert.False(t, ok)
}

func TestPostSamlResponse(t *testing.T) {
	w := httptest.NewRecorder()
	postSamlResponse(w, "https://sp.example.com/acs", "saml-response-data", "relay-state-value")

	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, `action="https://sp.example.com/acs"`)
	assert.Contains(t, body, `name="SAMLResponse"`)
	// The application layer already base64-encodes the response; the form must
	// post it verbatim (BE-04: a second encoding corrupts the assertion).
	assert.Contains(t, body, `value="saml-response-data"`)
	assert.Contains(t, body, `name="RelayState"`)
	assert.Contains(t, body, "document.forms[0].submit()")
}

func TestErrBadRequest_Saml(t *testing.T) {
	err := errBadRequest("test error")
	assert.Equal(t, "test error", err.Error())
}

func TestHandleSamlSSO_MissingApplicationID(t *testing.T) {
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/saml/sso/", nil)
	w := httptest.NewRecorder()
	h.handleSamlSSO(w, r)
	// Should return an error response
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandleSamlSSO_MissingSAMLRequest(t *testing.T) {
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/saml/sso/app1", nil)
	w := httptest.NewRecorder()
	h.handleSamlSSO(w, r)
	// Should return an error about missing SAMLRequest
	body := w.Body.String()
	assert.True(t, strings.Contains(body, "SAMLRequest") || w.Code != http.StatusOK)
}

func TestHandleSamlSSO_PostInvalidForm(t *testing.T) {
	h := Handler{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/saml/sso/app1", strings.NewReader("invalid"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.handleSamlSSO(w, r)
	// Should handle gracefully
}
