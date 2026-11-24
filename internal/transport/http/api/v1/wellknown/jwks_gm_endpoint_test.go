package wellknown

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	domainkeys "github.com/guliangiam/guliangiam/internal/domain/keys"
	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fetchJWKSDocument fetches a JWKS endpoint without requiring a non-empty key
// set: a single-family key ring legitimately publishes an empty document on
// the other family's endpoint.
func fetchJWKSDocument(t *testing.T, handler Handler, path string) (*http.Response, []rpJWK) {
	t.Helper()

	router := testRouter()
	RegisterChiRoutes(router, handler)
	resp := doRequest(t, router, http.MethodGet, path, nil)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}
	var set struct {
		Keys []rpJWK `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(body, &set))
	return resp, set.Keys
}

func gmGuardRSAPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func gmGuardRingManager(t *testing.T, entries ...map[string]any) *platformjwks.Manager {
	t.Helper()
	ringJSON, err := json.Marshal(map[string]any{"keys": entries})
	require.NoError(t, err)
	ring, err := platformjwks.ParseKeyRing(ringJSON)
	require.NoError(t, err)
	manager, err := platformjwks.NewManagerWithKeyRing(ring)
	require.NoError(t, err)
	return manager
}

func gmGuardRingEntry(kid, algorithm, privateKeyPEM string) map[string]any {
	return map[string]any{
		"kid":             kid,
		"algorithm":       algorithm,
		"private_key_pem": privateKeyPEM,
		"created_at":      time.Now().UTC(),
	}
}

// TestStandardJWKSEndpointHasNoSM2Curve is the regression that motivated the
// endpoint split: a strict JOSE library (Nimbus/Spring) aborts the whole key
// set on the unknown SM2 curve, so the standard jwks_uri must never carry a
// crv=SM2 key.
func TestStandardJWKSEndpointHasNoSM2Curve(t *testing.T) {
	fixture := newRPFixture(t)

	require.NotEmpty(t, fixture.standardKeys)
	for _, key := range fixture.standardKeys {
		assert.NotEqual(t, "SM2", key.Crv, "standard jwks.json leaked a GM key: %+v", key)
		assert.NotEqual(t, platformjwks.SigningAlgorithmSM2SM3, key.Alg)
		assert.False(t, strings.HasPrefix(key.Kid, rpKidPrefixGM),
			"standard jwks.json leaked a gm_ kid: %+v", key)
	}
}

// TestGMJWKSEndpointServesSM2Keys pins the dedicated GM channel: it answers
// 200 with the SM2-SM3 keys a GM-capable RP needs, cacheable like the standard
// endpoint.
func TestGMJWKSEndpointServesSM2Keys(t *testing.T) {
	fixture := newRPFixture(t)

	resp, keys := fetchJWKSDocument(t, fixture.handler, "/.well-known/jwks-gm.json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Cache-Control"), "max-age=3600")
	require.NotEmpty(t, keys)

	gmCount := 0
	for _, key := range keys {
		assert.Equal(t, platformjwks.SigningAlgorithmSM2SM3, key.Alg)
		assert.True(t, strings.HasPrefix(key.Kid, rpKidPrefixGM))
		if key.Crv == "SM2" {
			gmCount++
			assert.Equal(t, "EC", key.Kty)
			assert.NotEmpty(t, key.X)
			assert.NotEmpty(t, key.Y)
		}
	}
	assert.Positive(t, gmCount, "GM endpoint published no crv=SM2 key: %+v", keys)
}

// TestApplicationStandardJWKSHasNoSM2Curve extends the no-SM2 guarantee to the
// per-application jwks_uri, which merges the platform international keys.
func TestApplicationStandardJWKSHasNoSM2Curve(t *testing.T) {
	manager := platformjwks.NewManager(2048, nil)
	handler := Handler{
		JWKSManager:        rpJWKSProvider{manager: manager},
		OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{application: &appauth.OIDCApplication{Slug: "app1", OrganizationID: "org-1"}},
		KeyService: keyServiceWithKeys(t, "org-1", &domainkeys.Key{
			ID:             "app-key-1",
			OrganizationID: "org-1",
			Name:           "app-signing-app1",
			KeyType:        "rsa",
			PublicKey:      testRSAPublicKeyPEM(t),
			Algorithm:      platformjwks.SigningAlgorithmRS256,
			IsEnabled:      true,
		}),
	}

	resp, keys := fetchJWKSDocument(t, handler, "/.well-known/app1/jwks.json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, keys)
	for _, key := range keys {
		assert.NotEqual(t, "SM2", key.Crv, "application jwks.json leaked a GM key: %+v", key)
		assert.NotEqual(t, platformjwks.SigningAlgorithmSM2SM3, key.Alg)
		assert.False(t, strings.HasPrefix(key.Kid, rpKidPrefixGM))
	}
}

// TestDiscoveryAdvertisesGMJWKSEndpoint pins the discovery contract: GM-capable
// clients find the GM key set through gm_jwks_uri, and that URI must be the
// exact path the router declares and serves.
func TestDiscoveryAdvertisesGMJWKSEndpoint(t *testing.T) {
	fixture := newRPFixture(t)

	router := testRouter()
	RegisterChiRoutes(router, fixture.handler)
	resp := doRequest(t, router, http.MethodGet, "/.well-known/openid-configuration", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	var doc struct {
		Issuer                           string   `json:"issuer"`
		JWKSURI                          string   `json:"jwks_uri"`
		GMJWKSURI                        string   `json:"gm_jwks_uri"`
		GMAlgValuesSupported             []string `json:"gm_alg_values_supported"`
		IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	}
	require.NoError(t, json.Unmarshal(body, &doc))

	assert.Equal(t, fixture.handler.IssuerURL, doc.Issuer)
	assert.Equal(t, doc.Issuer+"/.well-known/jwks.json", doc.JWKSURI)
	assert.Equal(t, doc.Issuer+"/.well-known/jwks-gm.json", doc.GMJWKSURI)
	assert.Equal(t, []string{platformjwks.SigningAlgorithmSM2SM3}, doc.GMAlgValuesSupported)
	assert.Equal(t, []string{
		platformjwks.SigningAlgorithmRS256,
		platformjwks.SigningAlgorithmSM2SM3,
	}, doc.IDTokenSigningAlgValuesSupported)

	// The advertised GM path must match a declared GET route, and that route
	// must actually serve the GM document.
	gmPath := strings.TrimPrefix(doc.GMJWKSURI, doc.Issuer)
	declared := false
	for _, def := range Routes() {
		if def.Method == http.MethodGet && def.Pattern == gmPath {
			declared = true
			break
		}
	}
	assert.True(t, declared, "gm_jwks_uri path %q is not declared in Routes()", gmPath)

	gmResp, gmKeys := fetchJWKSDocument(t, fixture.handler, gmPath)
	require.Equal(t, http.StatusOK, gmResp.StatusCode)
	require.NotEmpty(t, gmKeys)
}

// TestFamilyEndpointsWithSingleFamilyRings pins the key-ring publication rule
// over HTTP: each endpoint publishes only its configured family, and an absent
// family is an empty document, never a generated key or an error.
func TestFamilyEndpointsWithSingleFamilyRings(t *testing.T) {
	rsaPEM := gmGuardRSAPrivateKeyPEM(t)
	sm2PEM, err := cryptotest.GenerateSM2PrivateKeyPEM()
	require.NoError(t, err)

	t.Run("rsa-only ring", func(t *testing.T) {
		manager := gmGuardRingManager(t,
			gmGuardRingEntry(rpKidPrefixRSA+"only", platformjwks.SigningAlgorithmRS256, rsaPEM))
		handler := Handler{JWKSManager: rpJWKSProvider{manager: manager}}

		resp, keys := fetchJWKSDocument(t, handler, "/.well-known/jwks.json")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Len(t, keys, 1)
		assert.Equal(t, platformjwks.SigningAlgorithmRS256, keys[0].Alg)

		resp, keys = fetchJWKSDocument(t, handler, "/.well-known/jwks-gm.json")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Empty(t, keys, "GM endpoint must be empty for an RSA-only ring")
	})

	t.Run("gm-only ring", func(t *testing.T) {
		manager := gmGuardRingManager(t,
			gmGuardRingEntry(rpKidPrefixGM+"only", platformjwks.SigningAlgorithmSM2SM3, sm2PEM))
		handler := Handler{JWKSManager: rpJWKSProvider{manager: manager}}

		resp, keys := fetchJWKSDocument(t, handler, "/.well-known/jwks.json")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Empty(t, keys, "standard endpoint must be empty for a GM-only ring")

		resp, keys = fetchJWKSDocument(t, handler, "/.well-known/jwks-gm.json")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Len(t, keys, 1)
		assert.Equal(t, "SM2", keys[0].Crv)
		assert.Equal(t, platformjwks.SigningAlgorithmSM2SM3, keys[0].Alg)
	})
}
