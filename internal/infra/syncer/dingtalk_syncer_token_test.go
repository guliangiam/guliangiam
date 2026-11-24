package syncer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dingTalkTokenTestRig retargets the package-level token endpoints at
// httptest servers and injects a loopback-allowlisted SSRF client.
type dingTalkTokenTestRig struct {
	syncer       *DingTalkSyncer
	oauthServer  *httptest.Server
	legacyServer *httptest.Server
	oauthCalls   int
	legacyCalls  int
}

func newDingTalkTokenTestRig(t *testing.T, oauthHandler, legacyHandler http.HandlerFunc) *dingTalkTokenTestRig {
	t.Helper()
	rig := &dingTalkTokenTestRig{}
	rig.oauthServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rig.oauthCalls++
		oauthHandler(w, r)
	}))
	rig.legacyServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rig.legacyCalls++
		legacyHandler(w, r)
	}))

	prevOAuth, prevLegacy := dingTalkOAuthTokenURL, dingTalkLegacyTokenURL
	dingTalkOAuthTokenURL = rig.oauthServer.URL
	dingTalkLegacyTokenURL = rig.legacyServer.URL
	t.Cleanup(func() {
		dingTalkOAuthTokenURL, dingTalkLegacyTokenURL = prevOAuth, prevLegacy
		rig.oauthServer.Close()
		rig.legacyServer.Close()
	})

	rig.syncer = &DingTalkSyncer{
		httpClient: ssrf.SafeHTTPClientWithAllowlist(5*time.Second, []string{"127.0.0.0/8"}),
	}
	return rig
}

func dingTalkTestConfig() *SyncerConfig {
	return &SyncerConfig{ClientID: "ding-app-key", ClientSecret: "ding-app-secret"}
}

// Official contract: POST JSON {appKey, appSecret} to
// api.dingtalk.com/v1.0/oauth2/accessToken, success {accessToken, expireIn}.
func TestDingTalkSyncer_OAuthTokenRequestShapeAndSuccess(t *testing.T) {
	rig := newDingTalkTokenTestRig(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var req map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "ding-app-key", req["appKey"])
		assert.Equal(t, "ding-app-secret", req["appSecret"])
		// expireIn is advisory and must not be hardcoded by the client.
		writeJSON(w, map[string]any{"accessToken": "dt-oauth-abc", "expireIn": 300})
	}, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("legacy endpoint must not be called on OAuth success")
		writeJSON(w, map[string]any{"errcode": 0, "access_token": "dt-legacy"})
	})

	token, err := rig.syncer.getDingTalkAccessToken(t.Context(), dingTalkTestConfig())
	require.NoError(t, err)
	assert.Equal(t, "dt-oauth-abc", token)
	assert.Equal(t, 1, rig.oauthCalls)
	assert.Equal(t, 0, rig.legacyCalls)
}

// HTTP 400 with a parseable error code is a business rejection: surfaced
// without legacy fallback (both endpoints share the credential store).
func TestDingTalkSyncer_OAuthBusinessErrorNoFallback(t *testing.T) {
	rig := newDingTalkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]any{"code": "InvalidAuthentication", "message": "appSecret mismatch", "requestid": "req-1"})
	}, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("legacy fallback must not run for business errors")
		writeJSON(w, map[string]any{"errcode": 0, "access_token": "dt-legacy"})
	})

	_, err := rig.syncer.getDingTalkAccessToken(t.Context(), dingTalkTestConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidAuthentication")
	assert.Contains(t, err.Error(), "appSecret mismatch")
	assert.Equal(t, 1, rig.oauthCalls)
	assert.Equal(t, 0, rig.legacyCalls)
}

// Non-business HTTP failure of the new endpoint falls back to the legacy
// gettoken with its query-parameter contract.
func TestDingTalkSyncer_OAuthHTTP404FallsBackToLegacy(t *testing.T) {
	rig := newDingTalkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "ding-app-key", r.URL.Query().Get("appkey"))
		assert.Equal(t, "ding-app-secret", r.URL.Query().Get("appsecret"))
		writeJSON(w, map[string]any{"errcode": 0, "errmsg": "ok", "access_token": "dt-legacy-xyz", "expires_in": 7200})
	})

	token, err := rig.syncer.getDingTalkAccessToken(t.Context(), dingTalkTestConfig())
	require.NoError(t, err)
	assert.Equal(t, "dt-legacy-xyz", token)
	assert.Equal(t, 1, rig.oauthCalls)
	assert.Equal(t, 1, rig.legacyCalls)
}

// Transport failure of the new endpoint also falls back (multi-version /
// private-deployment compatibility).
func TestDingTalkSyncer_OAuthTransportFailureFallsBackToLegacy(t *testing.T) {
	rig := newDingTalkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"accessToken": "unreachable"})
	}, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"errcode": 0, "access_token": "dt-legacy-fallback"})
	})
	rig.oauthServer.Close()

	token, err := rig.syncer.getDingTalkAccessToken(t.Context(), dingTalkTestConfig())
	require.NoError(t, err)
	assert.Equal(t, "dt-legacy-fallback", token)
	assert.Equal(t, 1, rig.legacyCalls)
}

// Both endpoints failing surfaces the legacy error after fallback.
func TestDingTalkSyncer_BothEndpointsFail(t *testing.T) {
	rig := newDingTalkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"errcode": 40089, "errmsg": "invalid appsecret"})
	})

	_, err := rig.syncer.getDingTalkAccessToken(t.Context(), dingTalkTestConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "40089")
	assert.Equal(t, 1, rig.oauthCalls)
	assert.Equal(t, 1, rig.legacyCalls)
}

// HTTP 200 without accessToken is a malformed success: error, no fallback.
func TestDingTalkSyncer_OAuthMissingAccessTokenNoFallback(t *testing.T) {
	rig := newDingTalkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"expireIn": 7200})
	}, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("legacy fallback must not run for malformed success shape")
		writeJSON(w, map[string]any{"errcode": 0, "access_token": "dt-legacy"})
	})

	_, err := rig.syncer.getDingTalkAccessToken(t.Context(), dingTalkTestConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accessToken")
	assert.Equal(t, 0, rig.legacyCalls)
}

// Explicit APIURL keeps the legacy query-parameter contract for custom
// deployments; the default OAuth endpoint is not consulted.
func TestDingTalkSyncer_APIURLOverrideUsesLegacyContract(t *testing.T) {
	rig := newDingTalkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("default OAuth endpoint must not be called with APIURL override")
		writeJSON(w, map[string]any{"accessToken": "dt-oauth"})
	}, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/gettoken", r.URL.Path)
		assert.Equal(t, "ding-app-key", r.URL.Query().Get("appkey"))
		writeJSON(w, map[string]any{"errcode": 0, "access_token": "dt-custom"})
	})

	config := dingTalkTestConfig()
	config.APIURL = rig.legacyServer.URL
	token, err := rig.syncer.getDingTalkAccessToken(t.Context(), config)
	require.NoError(t, err)
	assert.Equal(t, "dt-custom", token)
	assert.Equal(t, 0, rig.oauthCalls)
	assert.Equal(t, 1, rig.legacyCalls)
}

func TestDingTalkSyncer_TestConnectionWrapsTokenError(t *testing.T) {
	rig := newDingTalkTokenTestRig(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]any{"code": "Forbidden.AccessDenied", "message": "app not authorized"})
	}, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("legacy fallback must not run for business errors")
		writeJSON(w, map[string]any{"errcode": 0, "access_token": "dt-legacy"})
	})

	err := rig.syncer.TestConnection(t.Context(), dingTalkTestConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Forbidden.AccessDenied")
}
