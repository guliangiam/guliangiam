package syncer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// larkTokenTestRig retargets the package-level token endpoints at httptest
// servers and injects a loopback-allowlisted SSRF client (idp precedent).
type larkTokenTestRig struct {
	syncer       *LarkSyncer
	oauthServer  *httptest.Server
	legacyServer *httptest.Server
	oauthCalls   int
	legacyCalls  int
}

func newLarkTokenTestRig(t *testing.T, oauthHandler, legacyHandler http.HandlerFunc) *larkTokenTestRig {
	t.Helper()
	rig := &larkTokenTestRig{}
	rig.oauthServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rig.oauthCalls++
		oauthHandler(w, r)
	}))
	rig.legacyServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rig.legacyCalls++
		legacyHandler(w, r)
	}))

	prevOAuth, prevLegacy := larkOAuthTokenURL, larkLegacyTokenURL
	larkOAuthTokenURL = rig.oauthServer.URL
	larkLegacyTokenURL = rig.legacyServer.URL
	t.Cleanup(func() {
		larkOAuthTokenURL, larkLegacyTokenURL = prevOAuth, prevLegacy
		rig.oauthServer.Close()
		rig.legacyServer.Close()
	})

	rig.syncer = &LarkSyncer{
		httpClient: ssrf.SafeHTTPClientWithAllowlist(5*time.Second, []string{"127.0.0.0/8"}),
	}
	return rig
}

func larkTestConfig() *SyncerConfig {
	return &SyncerConfig{ClientID: "cli_app_id", ClientSecret: "app-secret"}
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

// Official contract: POST form-urlencoded grant_type=client_credentials with
// client_secret_post only (no HTTP Basic alongside, error 20070), success
// shape {code:0, access_token, expires_in, token_type}.
func TestLarkSyncer_OAuthTokenRequestShapeAndSuccess(t *testing.T) {
	rig := newLarkTokenTestRig(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		assert.Empty(t, r.Header.Get("Authorization"), "client_secret_basic must not accompany client_secret_post")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		assert.Equal(t, "client_credentials", form.Get("grant_type"))
		assert.Equal(t, "cli_app_id", form.Get("client_id"))
		assert.Equal(t, "app-secret", form.Get("client_secret"))
		// expires_in is advisory and must not be hardcoded by the client.
		writeJSON(w, map[string]any{"code": 0, "access_token": "t-oauth-abc", "expires_in": 120, "token_type": "Bearer"})
	}, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("legacy endpoint must not be called on OAuth success")
		writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "t-legacy"})
	})

	token, err := rig.syncer.getLarkAccessToken(t.Context(), larkTestConfig())
	require.NoError(t, err)
	assert.Equal(t, "t-oauth-abc", token)
	assert.Equal(t, 1, rig.oauthCalls)
	assert.Equal(t, 0, rig.legacyCalls)
}

// Failure shape {code!=0, error, error_description} (no msg field) must be
// surfaced; credential errors do not trigger the legacy fallback.
func TestLarkSyncer_OAuthBusinessErrorNoFallback(t *testing.T) {
	rig := newLarkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"code": 20002, "error": "invalid_client_secret", "error_description": "app secret mismatch"})
	}, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("legacy fallback must not run for business errors")
		writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "t-legacy"})
	})

	_, err := rig.syncer.getLarkAccessToken(t.Context(), larkTestConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "20002")
	assert.Contains(t, err.Error(), "app secret mismatch")
	assert.Equal(t, 1, rig.oauthCalls)
	assert.Equal(t, 0, rig.legacyCalls)
}

// code=0 without access_token is a malformed success: error, no fallback
// (the legacy tenant_access_token field must not be read from this endpoint).
func TestLarkSyncer_OAuthMissingAccessTokenNoLegacyFieldRead(t *testing.T) {
	rig := newLarkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "t-legacy-should-not-be-read", "expires_in": 7200})
	}, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("legacy fallback must not run for malformed success shape")
		writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "t-legacy"})
	})

	_, err := rig.syncer.getLarkAccessToken(t.Context(), larkTestConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_token")
	assert.Equal(t, 0, rig.legacyCalls)
}

// HTTP-level failure of the new endpoint falls back to the historical
// internal endpoint with its JSON app_id/app_secret contract.
func TestLarkSyncer_OAuthHTTP404FallsBackToLegacy(t *testing.T) {
	rig := newLarkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var req larkTokenRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "cli_app_id", req.AppID)
		assert.Equal(t, "app-secret", req.AppSecret)
		writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "t-legacy-xyz", "expire": 7200})
	})

	token, err := rig.syncer.getLarkAccessToken(t.Context(), larkTestConfig())
	require.NoError(t, err)
	assert.Equal(t, "t-legacy-xyz", token)
	assert.Equal(t, 1, rig.oauthCalls)
	assert.Equal(t, 1, rig.legacyCalls)
}

// Transport failure of the new endpoint also falls back (multi-version /
// private-deployment compatibility).
func TestLarkSyncer_OAuthTransportFailureFallsBackToLegacy(t *testing.T) {
	rig := newLarkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"code": 0, "access_token": "unreachable"})
	}, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "t-legacy-fallback"})
	})
	rig.oauthServer.Close()

	token, err := rig.syncer.getLarkAccessToken(t.Context(), larkTestConfig())
	require.NoError(t, err)
	assert.Equal(t, "t-legacy-fallback", token)
	assert.Equal(t, 1, rig.legacyCalls)
}

// Both endpoints failing surfaces the legacy error after fallback.
func TestLarkSyncer_BothEndpointsFail(t *testing.T) {
	rig := newLarkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"code": 10003, "msg": "invalid app_id"})
	})

	_, err := rig.syncer.getLarkAccessToken(t.Context(), larkTestConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10003")
	assert.Equal(t, 1, rig.oauthCalls)
	assert.Equal(t, 1, rig.legacyCalls)
}

// Explicit TokenURL keeps the legacy JSON contract for custom deployments;
// the default OAuth endpoint is not consulted.
func TestLarkSyncer_TokenURLOverrideUsesLegacyContract(t *testing.T) {
	rig := newLarkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("default OAuth endpoint must not be called with TokenURL override")
		writeJSON(w, map[string]any{"code": 0, "access_token": "t-oauth"})
	}, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var req larkTokenRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "cli_app_id", req.AppID)
		writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "t-custom"})
	})

	config := larkTestConfig()
	config.TokenURL = rig.legacyServer.URL
	token, err := rig.syncer.getLarkAccessToken(t.Context(), config)
	require.NoError(t, err)
	assert.Equal(t, "t-custom", token)
	assert.Equal(t, 0, rig.oauthCalls)
	assert.Equal(t, 1, rig.legacyCalls)
}

func TestLarkSyncer_TestConnectionWrapsTokenError(t *testing.T) {
	rig := newLarkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"code": 20048, "error": "app_not_exist", "error_description": "app does not exist"})
	}, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("legacy fallback must not run for business errors")
		writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "t-legacy"})
	})

	err := rig.syncer.TestConnection(t.Context(), larkTestConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lark test connection failed")
	assert.Contains(t, err.Error(), "20048")
}
