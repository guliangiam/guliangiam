package federation

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainworkload "github.com/guliangiam/guliangiam/internal/domain/workload"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signFederationTestJWT(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]any{"alg": "RS256", "kid": kid, "typ": "JWT"})
	require.NoError(t, err)
	payloadJSON, err := json.Marshal(claims)
	require.NoError(t, err)
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func newFederationTestVerifier(t *testing.T) (*Verifier, *rsa.PrivateKey, *domainworkload.FederationTrust, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "federation-key-1"
	jwk := platformjwks.JWK{
		Kty: "RSA", Use: "sig", Kid: kid, Alg: "RS256",
		N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
	body, err := json.Marshal(platformjwks.JWKS{Keys: []platformjwks.JWK{jwk}})
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	cache := platformjwks.NewRemoteJWKSCache(time.Hour, time.Hour, server.Client())
	t.Cleanup(cache.Close)
	trust := &domainworkload.FederationTrust{
		ID: "trust-1", OrganizationID: "org-1", Issuer: "https://issuer.example.com",
		JWKSURI: server.URL + "/jwks", Audience: []string{"guliangiam"},
		Subjects: []string{"repo:org/*"}, Status: domainworkload.TrustStatusActive,
	}
	return NewVerifier(cache), key, trust, kid
}

func validFederationTestClaims(issuer string) map[string]any {
	return map[string]any{
		"iss": issuer,
		"sub": "repo:org/api",
		"aud": "guliangiam",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"nbf": time.Now().Add(-time.Minute).Unix(),
	}
}

func TestVerifierAcceptsValidRS256JWT(t *testing.T) {
	verifier, key, trust, kid := newFederationTestVerifier(t)
	token := signFederationTestJWT(t, key, kid, validFederationTestClaims(trust.Issuer))
	claims, err := verifier.Verify(token, trust)
	require.NoError(t, err)
	assert.Equal(t, "repo:org/api", claims["sub"])
}

func TestVerifierFailClosedOnClaimsAndTrustConfiguration(t *testing.T) {
	verifier, key, trust, kid := newFederationTestVerifier(t)
	tests := []struct {
		name        string
		mutate      func(map[string]any)
		mutateTrust func(*domainworkload.FederationTrust)
		errorText   string
	}{
		{name: "audience mismatch", mutate: func(claims map[string]any) {
			claims["aud"] = "another-resource"
		}, errorText: "audience"},
		{name: "issuer mismatch", mutate: func(claims map[string]any) {
			claims["iss"] = "https://other-issuer.example.com"
		}, errorText: "issuer"},
		{name: "expired", mutate: func(claims map[string]any) {
			claims["exp"] = time.Now().Add(-time.Minute).Unix()
		}, errorText: "expired"},
		{name: "missing exp", mutate: func(claims map[string]any) {
			delete(claims, "exp")
		}, errorText: "missing exp"},
		{name: "future nbf", mutate: func(claims map[string]any) {
			claims["nbf"] = time.Now().Add(10 * time.Minute).Unix()
		}, errorText: "not yet valid"},
		{name: "invalid nbf", mutate: func(claims map[string]any) {
			claims["nbf"] = "not-a-number"
		}, errorText: "nbf"},
		{name: "empty trust audience", mutateTrust: func(trust *domainworkload.FederationTrust) {
			trust.Audience = nil
		}, errorText: "audience is empty"},
		{name: "blank trust audience", mutateTrust: func(trust *domainworkload.FederationTrust) {
			trust.Audience = []string{" "}
		}, errorText: "audience is empty"},
		{name: "disabled trust", mutateTrust: func(trust *domainworkload.FederationTrust) {
			trust.Status = domainworkload.TrustStatusDisabled
		}, errorText: "not active"},
		{name: "deleted trust", mutateTrust: func(trust *domainworkload.FederationTrust) {
			trust.Status = domainworkload.TrustStatusDeleted
		}, errorText: "not active"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := validFederationTestClaims(trust.Issuer)
			if test.mutate != nil {
				test.mutate(claims)
			}
			candidate := *trust
			if test.mutateTrust != nil {
				test.mutateTrust(&candidate)
			}
			token := signFederationTestJWT(t, key, kid, claims)
			result, err := verifier.Verify(token, &candidate)
			assert.Nil(t, result)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.errorText)
		})
	}
}

func TestVerifierRejectsBadSignatureAndUnknownKid(t *testing.T) {
	verifier, key, trust, kid := newFederationTestVerifier(t)
	claims := validFederationTestClaims(trust.Issuer)

	t.Run("bad signature", func(t *testing.T) {
		otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		token := signFederationTestJWT(t, otherKey, kid, claims)
		result, err := verifier.Verify(token, trust)
		assert.Nil(t, result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signature")
	})

	t.Run("unknown kid", func(t *testing.T) {
		token := signFederationTestJWT(t, key, "unknown-kid", claims)
		result, err := verifier.Verify(token, trust)
		assert.Nil(t, result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown signing key")
	})
}
