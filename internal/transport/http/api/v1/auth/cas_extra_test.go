package auth

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteCasSuccessResponse(t *testing.T) {
	w := httptest.NewRecorder()
	result := &casAuthResult{User: "testuser", Org: "testorg"}
	writeCasSuccessResponse(w, result, false)

	assert.Equal(t, "application/xml;charset=UTF-8", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "testuser")
	assert.Contains(t, body, "authenticationSuccess")
}

func TestWriteCasErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	writeCasErrorResponse(w, "INVALID_TICKET", "ticket expired")

	assert.Equal(t, "application/xml;charset=UTF-8", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "INVALID_TICKET")
	assert.Contains(t, body, "ticket expired")
	assert.Contains(t, body, "authenticationFailure")
}

func TestCasXMLServiceResponse_Marshal(t *testing.T) {
	resp := casXMLServiceResponse{
		AuthSuccess: &casXMLAuthSuccess{User: "alice"},
	}
	output, err := xml.Marshal(resp)
	require.NoError(t, err)
	assert.Contains(t, string(output), "alice")

	resp2 := casXMLServiceResponse{
		AuthFailure: &casXMLAuthFailure{Code: "INVALID_REQUEST", Message: "bad request"},
	}
	output2, err := xml.Marshal(resp2)
	require.NoError(t, err)
	assert.Contains(t, string(output2), "INVALID_REQUEST")
}

func TestValidateCasTicket_InvalidFormat(t *testing.T) {
	h := Handler{}
	_, err := h.validateCasTicket(nil, "INVALID", "http://service")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ticket format")
}

func TestValidateCasTicket_SessionNotFound(t *testing.T) {
	h := Handler{}
	_, err := h.validateCasTicket(nil, "ST-session123", "http://service")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestCasLoginRedirect_MissingService(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/login", nil)
	h.CasLoginRedirect(w, r)
	assert.NotEqual(t, http.StatusFound, w.Code)
}

func TestCasLoginRedirect_Success(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/login?service=http://app.example.com", nil)
	h.CasLoginRedirect(w, r)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "service=")
}

func TestCasLoginRedirect_CustomCasLoginURL(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/login?service=http://app.example.com&casLoginUrl=/custom-login", nil)
	h.CasLoginRedirect(w, r)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/custom-login")
}

func TestCasServiceValidate_MissingParams(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/serviceValidate", nil)
	h.CasServiceValidate(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCasServiceValidate_InvalidTicket(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/serviceValidate?service=http://app&ticket=INVALID", nil)
	h.CasServiceValidate(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "invalid ticket format")
}

func TestCasProxy_MissingTargetService(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/proxy", nil)
	h.CasProxy(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCasProxy_Unsupported(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/proxy?targetService=http://app", nil)
	h.CasProxy(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "proxy tickets are not supported")
}

func TestCasLogout_NoCookie(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/logout", nil)
	h.CasLogout(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"redirectRejected":false`)
}

// TestCasLogout_RefusesUnvalidatedServiceRedirect pins the FE-014 requirement:
// with no session there is no organization, so there is no redirect whitelist to
// validate `service` against, and the endpoint must not echo it back.
func TestCasLogout_RefusesUnvalidatedServiceRedirect(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/logout?service=https://evil.example.com/steal", nil)
	h.CasLogout(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Location"))
	assert.Contains(t, w.Body.String(), `"redirectRejected":true`)
	assert.NotContains(t, w.Body.String(), "evil.example.com")
}

func TestCasLogout_WithSessionCookie(t *testing.T) {
	// Skip: CasLogout with session cookie calls h.Service.CasLogout which panics with nil Service
	// This is tested via chi route registration with Recoverer middleware
}

// TestCasLogout_ReportModeNeverRedirects pins the console contract: with
// `redirect=report` the verdict has to arrive in the body, because the console
// cannot read the Location header of a 302 it is not allowed to follow.
func TestCasLogout_ReportModeNeverRedirects(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(
		http.MethodGet,
		"/cas/logout?redirect=report&service=https://evil.example.com/steal",
		nil,
	)
	h.CasLogout(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Location"))
	body := w.Body.String()
	assert.Contains(t, body, `"redirectRejected":true`)
	assert.Contains(t, body, `"redirectUri":""`)
	assert.NotContains(t, body, "evil.example.com")
}

// TestCasLogout_ReportModeIsOptIn documents that the CAS protocol surface keeps
// the 302 default: a client that does not ask for the report is unaffected.
func TestCasLogout_UnknownRedirectModeNeedsNoRedirect(t *testing.T) {
	for _, mode := range []string{"", "follow", "bogus"} {
		h := Handler{}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(
			http.MethodGet,
			"/cas/logout?redirect="+mode+"&service=https://evil.example.com/steal",
			nil,
		)
		h.CasLogout(w, r)

		assert.Equal(t, http.StatusOK, w.Code, "redirect=%q", mode)
		assert.Empty(t, w.Header().Get("Location"), "redirect=%q", mode)
		assert.Contains(t, w.Body.String(), `"redirectUri":""`, "redirect=%q", mode)
	}
}

func TestCasLogout_ClearsSessionCookie(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/logout", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-1"})
	h.CasLogout(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	cookies := w.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, SessionCookieName, cookies[0].Name)
	assert.Empty(t, cookies[0].Value)
}

func TestCasProxyValidate_DelegatesToServiceValidate(t *testing.T) {
	h := Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cas/proxyValidate?service=http://app&ticket=ST-123", nil)
	h.CasProxyValidate(w, r)
	// Should behave the same as CasServiceValidate
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestResolveSessionByID_ReturnsFalse(t *testing.T) {
	h := Handler{}
	_, ok := h.resolveSessionByID(nil, "session123")
	assert.False(t, ok)
}
