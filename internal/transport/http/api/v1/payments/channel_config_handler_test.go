package payments

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newChannelConfigTestCipher(t *testing.T) *platformcrypto.FieldEncryptor {
	t.Helper()
	t.Setenv("ENCRYPTION_ALGORITHM", "aes-256-gcm")
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("ENCRYPTION_KEY_COMPAT", "")

	encryptor, err := platformcrypto.NewFieldEncryptorChecked()
	require.NoError(t, err)
	return encryptor
}

func channelConfigRouter(service apppayments.ChannelConfigService) http.Handler {
	router := testRouter()
	RegisterChannelConfigChiRoutes(router, NewChannelConfigHandler(service))
	return router
}

func postChannelConfig(t *testing.T, router http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := withTrustedTenant(httptest.NewRequest(http.MethodPost, "/api/v1/pay-channel-configs", strings.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestChannelConfigCreate_SealsPlaintextConfigServerSide(t *testing.T) {
	const plaintextSecret = "sk_live_a6_2b_canary"
	repo := newTenantChannelConfigsRepo()
	service := apppayments.NewChannelConfigService(repo, tenantSystemClock{}, newChannelConfigTestCipher(t), platformuuid.V7Generator{})
	router := channelConfigRouter(service)

	recorder := postChannelConfig(t, router, `{
		"id":"cfg-1",
		"channel":"stripe",
		"environment":"sandbox",
		"config":{"appId":"acct_1","privateKey":"`+plaintextSecret+`"}
	}`)
	require.Equal(t, http.StatusCreated, recorder.Code)

	stored := repo.byID["cfg-1"]
	require.NotNil(t, stored)
	assert.True(t, platformcrypto.IsEnvelopeToken(stored.ConfigSealed),
		"config_sealed must be a v1/v2 envelope, got %q", stored.ConfigSealed)
	assert.True(t, strings.HasPrefix(stored.ConfigSealed, "v1-"),
		"DecryptField readers require the v1 sealed-payload envelope")
	assert.NotContains(t, stored.ConfigSealed, plaintextSecret)
	assert.Equal(t, []string{"appId", "privateKey"}, stored.ConfigSet,
		"configSet must be derived from the plaintext keys, not supplied by the client")

	body := recorder.Body.String()
	assert.NotContains(t, body, plaintextSecret)
	assert.NotContains(t, body, stored.ConfigSealed)
	assert.NotContains(t, body, `"config":`)
	assert.NotContains(t, body, `"configSealed"`)
}

func TestChannelConfigCreate_GeneratesIDWhenOmitted(t *testing.T) {
	repo := newTenantChannelConfigsRepo()
	service := apppayments.NewChannelConfigService(repo, tenantSystemClock{}, newChannelConfigTestCipher(t), platformuuid.V7Generator{})
	router := channelConfigRouter(service)

	recorder := postChannelConfig(t, router, `{
		"channel":"square",
		"environment":"sandbox",
		"config":{"accessToken":"t","locationId":"L"}
	}`)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Len(t, repo.byID, 1)
	for id := range repo.byID {
		assert.True(t, platformuuid.IsValid(id), "server-generated id must be a UUID: %s", id)
	}
}

func TestChannelConfigCreate_RejectsLegacyAndInvalidPayloads(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			"legacy configSealed",
			`{"id":"cfg-1","channel":"stripe","configSealed":"v1-aes:AAAA:BBBB:CCCC"}`,
		},
		{
			"legacy configSet",
			`{"id":"cfg-1","channel":"stripe","config":{"appId":"a"},"configSet":["appId"]}`,
		},
		{
			"missing config",
			`{"id":"cfg-1","channel":"stripe"}`,
		},
		{
			"empty object",
			`{"id":"cfg-1","channel":"stripe","config":{}}`,
		},
		{
			"null config",
			`{"id":"cfg-1","channel":"stripe","config":null}`,
		},
		{
			"array config",
			`{"id":"cfg-1","channel":"stripe","config":["a"]}`,
		},
		{
			"string config",
			`{"id":"cfg-1","channel":"stripe","config":"{\"a\":1}"}`,
		},
		{
			"malformed config",
			`{"id":"cfg-1","channel":"stripe","config":{"a":`,
		},
		{
			"oversize config",
			`{"id":"cfg-1","channel":"stripe","config":{"a":"` + strings.Repeat("x", 64<<10) + `"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTenantChannelConfigsRepo()
			service := apppayments.NewChannelConfigService(repo, tenantSystemClock{}, newChannelConfigTestCipher(t), platformuuid.V7Generator{})
			recorder := postChannelConfig(t, channelConfigRouter(service), tt.body)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Empty(t, repo.byID, "rejected payloads must not reach the repository")
			assert.NotContains(t, recorder.Body.String(), "v1-aes:AAAA:BBBB:CCCC")
		})
	}
}
