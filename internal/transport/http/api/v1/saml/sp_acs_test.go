package saml

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type fakeSPACSCompleter struct {
	command *appauth.CompleteSAMLSocialLoginCommand
	result  *appauth.SocialLoginResult
	err     error
}

func (f *fakeSPACSCompleter) CompleteSAMLSocialLogin(_ context.Context, command appauth.CompleteSAMLSocialLoginCommand) (*appauth.SocialLoginResult, error) {
	f.command = &command
	return f.result, f.err
}

func spACSRequest(t *testing.T, path string, form url.Values) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestRegisterPublicChiRoutes_FormParsingAndRedirect(t *testing.T) {
	completer := &fakeSPACSCompleter{
		result: &appauth.SocialLoginResult{
			LoginResult: appauth.LoginResult{
				SessionID: "session-1",
				Response:  appauth.LoginResponse{RedirectTo: "/"},
			},
		},
	}
	handler := &Handler{SPACSCompleter: completer}
	router := testRouter()
	RegisterPublicChiRoutes(router, handler)

	form := url.Values{
		"SAMLResponse": {"saml-response"},
		"RelayState":   {"signed-state"},
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, spACSRequest(t, "/api/v1/auth/saml/acs/test-org/01a00000-0000-7000-8000-0000000000b1", form))
	resp := rec.Result()

	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
	require.Len(t, resp.Cookies(), 1)
	assert.Equal(t, "session-1", resp.Cookies()[0].Value)

	require.NotNil(t, completer.command)
	assert.Equal(t, "test-org", completer.command.OrganizationSlug)
	assert.Equal(t, "01a00000-0000-7000-8000-0000000000b1", completer.command.ProviderID)
	assert.Equal(t, "saml-response", completer.command.SAMLResponse)
	assert.Equal(t, "signed-state", completer.command.RelayState)
}

func TestRegisterPublicChiRoutes_RejectsMissingForm(t *testing.T) {
	router := testRouter()
	RegisterPublicChiRoutes(router, &Handler{SPACSCompleter: &fakeSPACSCompleter{}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, spACSRequest(t, "/api/v1/auth/saml/acs/test-org/01a00000-0000-7000-8000-0000000000b1", url.Values{}))
	assert.Equal(t, http.StatusBadRequest, rec.Result().StatusCode)
}

func TestRegisterPublicChiRoutes_ErrorMappingDoesNotEchoAssertion(t *testing.T) {
	completer := &fakeSPACSCompleter{err: apperrors.NewUnauthorized("SAML state is missing, expired, or already used")}
	router := testRouter()
	RegisterPublicChiRoutes(router, &Handler{SPACSCompleter: completer})

	form := url.Values{
		"SAMLResponse": {"<Assertion>very-secret</Assertion>"},
		"RelayState":   {"signed-state"},
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, spACSRequest(t, "/api/v1/auth/saml/acs/test-org/01a00000-0000-7000-8000-0000000000b1", form))
	resp := rec.Result()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "invalid_token", body["error"])
	assert.NotContains(t, body["error_description"], "Assertion")
	assert.NotContains(t, body["error_description"], "SAMLResponse")
}

func TestRegisterPublicChiRoutes_RejectsOversizedResponse(t *testing.T) {
	completer := &fakeSPACSCompleter{}
	router := testRouter()
	RegisterPublicChiRoutes(router, &Handler{SPACSCompleter: completer})

	form := url.Values{
		"SAMLResponse": {strings.Repeat("a", maxSPACSResponseBytes+1)},
		"RelayState":   {"signed-state"},
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, spACSRequest(t, "/api/v1/auth/saml/acs/test-org/01a00000-0000-7000-8000-0000000000b1", form))
	assert.Equal(t, http.StatusBadRequest, rec.Result().StatusCode)
	assert.Nil(t, completer.command)
}

func TestRegisterPublicChiRoutes_RejectsOversizedRelayState(t *testing.T) {
	completer := &fakeSPACSCompleter{}
	router := testRouter()
	RegisterPublicChiRoutes(router, &Handler{SPACSCompleter: completer})

	form := url.Values{
		"SAMLResponse": {"saml-response"},
		"RelayState":   {strings.Repeat("a", maxSPACSRelayStateBytes+1)},
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, spACSRequest(t, "/api/v1/auth/saml/acs/test-org/01a00000-0000-7000-8000-0000000000b1", form))
	assert.Equal(t, http.StatusBadRequest, rec.Result().StatusCode)
	assert.Nil(t, completer.command)
}

func TestSPACSRouteIsOnlyOnThePublicGroup(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, &Handler{AuthService: &appauth.Service{}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, spACSRequest(t, "/api/v1/auth/saml/acs/test-org/01a00000-0000-7000-8000-0000000000b1", url.Values{
		"SAMLResponse": {"saml-response"},
		"RelayState":   {"signed-state"},
	}))
	assert.Equal(t, http.StatusNotFound, rec.Result().StatusCode, "the protected SAML group must not mount the SP ACS")
}

func TestRegisterPublicChiRoutes_OnlyAcceptsPost(t *testing.T) {
	router := testRouter()
	RegisterPublicChiRoutes(router, &Handler{SPACSCompleter: &fakeSPACSCompleter{}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/saml/acs/test-org/01a00000-0000-7000-8000-0000000000b1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Result().StatusCode)
}
