package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJWTClaimsUnverified(t *testing.T) {
	t.Run("valid_token", func(t *testing.T) {
		payload := map[string]interface{}{
			"iss": "test-issuer",
			"sub": "test-subject",
			"aud": "test-audience",
		}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)
		encoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
		token := "header." + encoded + ".signature"

		claims, err := parseJWTClaimsUnverified(token)
		assert.NoError(t, err)
		assert.Equal(t, "test-issuer", claims["iss"])
		assert.Equal(t, "test-subject", claims["sub"])
		assert.Equal(t, "test-audience", claims["aud"])
	})

	t.Run("invalid_format_no_dots", func(t *testing.T) {
		_, err := parseJWTClaimsUnverified("invalidtoken")
		assert.Error(t, err)
	})

	t.Run("invalid_format_two_parts", func(t *testing.T) {
		_, err := parseJWTClaimsUnverified("header.payload")
		assert.Error(t, err)
	})

	t.Run("invalid_base64_payload", func(t *testing.T) {
		_, err := parseJWTClaimsUnverified("header.!!!invalid!!!.signature")
		assert.Error(t, err)
	})

	t.Run("invalid_json_payload", func(t *testing.T) {
		encoded := base64.RawURLEncoding.EncodeToString([]byte("not json"))
		token := "header." + encoded + ".signature"
		_, err := parseJWTClaimsUnverified(token)
		assert.Error(t, err)
	})
}

func TestParseRSAPublicKeyFromPEM(t *testing.T) {
	t.Run("valid_certificate", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		template := &x509.Certificate{
			SerialNumber: big.NewInt(1),
		}
		certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
		require.NoError(t, err)

		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

		pubKey, err := parseRSAPublicKeyFromPEM(string(certPEM))
		assert.NoError(t, err)
		assert.NotNil(t, pubKey)
		assert.Equal(t, &privateKey.PublicKey, pubKey)
	})

	t.Run("valid_pkix_public_key", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		pubDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		require.NoError(t, err)

		pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

		pubKey, err := parseRSAPublicKeyFromPEM(string(pubPEM))
		assert.NoError(t, err)
		assert.NotNil(t, pubKey)
		assert.Equal(t, &privateKey.PublicKey, pubKey)
	})

	t.Run("invalid_pem", func(t *testing.T) {
		_, err := parseRSAPublicKeyFromPEM("not a pem")
		assert.Error(t, err)
	})

	t.Run("empty_string", func(t *testing.T) {
		_, err := parseRSAPublicKeyFromPEM("")
		assert.Error(t, err)
	})
}
