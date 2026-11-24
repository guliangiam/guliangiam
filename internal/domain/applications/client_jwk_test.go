package applications

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var (
	testRSAOnce sync.Once
	testRSAKey  *rsa.PrivateKey
	testRSAErr  error
)

func testRSA2048(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	testRSAOnce.Do(func() {
		testRSAKey, testRSAErr = rsa.GenerateKey(rand.Reader, 2048)
	})
	require.NoError(t, testRSAErr)
	return testRSAKey
}

func rsaJWK(t *testing.T, key *rsa.PrivateKey, kid, alg string, extra map[string]any) map[string]any {
	t.Helper()
	members := map[string]any{
		"kty": "RSA",
		"kid": kid,
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
	if alg != "" {
		members["alg"] = alg
	}
	for k, v := range extra {
		members[k] = v
	}
	return members
}

func ecJWK(t *testing.T, key *ecdsa.PrivateKey, kid, alg string, extra map[string]any) map[string]any {
	t.Helper()
	members := map[string]any{
		"kty": "EC",
		"kid": kid,
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(key.X.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(key.Y.Bytes()),
	}
	if alg != "" {
		members["alg"] = alg
	}
	for k, v := range extra {
		members[k] = v
	}
	return members
}

func marshalJWKS(t *testing.T, keys ...any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"keys": keys})
	require.NoError(t, err)
	return raw
}

func assertJWKInvalid(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, 422, appErr.HTTPStatus())
	assert.Equal(t, "jwks", appErr.Meta["field"])
}

func TestParseClientJWKS_AcceptsPublicRSAAndEC(t *testing.T) {
	rsaKey := testRSA2048(t)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	t.Run("rsa_rs256", func(t *testing.T) {
		parsed, err := ParseClientJWKS(marshalJWKS(t, rsaJWK(t, rsaKey, "rsa-1", "RS256", nil)))
		require.NoError(t, err)
		require.Len(t, parsed, 1)
		assert.Equal(t, "rsa-1", parsed[0].Kid)
		assert.Equal(t, "RS256", parsed[0].Alg)

		var canonical map[string]any
		require.NoError(t, json.Unmarshal([]byte(parsed[0].JWK), &canonical))
		assert.Equal(t, "RSA", canonical["kty"])
		assert.Equal(t, "rsa-1", canonical["kid"])
		assert.NotContains(t, canonical, "d", "落库的公钥不得保留私钥参数")
	})

	t.Run("rsa_ps256", func(t *testing.T) {
		parsed, err := ParseClientJWKS(marshalJWKS(t, rsaJWK(t, rsaKey, "rsa-ps", "PS256", nil)))
		require.NoError(t, err)
		require.Len(t, parsed, 1)
		assert.Equal(t, "PS256", parsed[0].Alg)
	})

	t.Run("ec_es256", func(t *testing.T) {
		parsed, err := ParseClientJWKS(marshalJWKS(t, ecJWK(t, ecKey, "ec-1", "ES256", nil)))
		require.NoError(t, err)
		require.Len(t, parsed, 1)
		assert.Equal(t, "ec-1", parsed[0].Kid)
		assert.Equal(t, "ES256", parsed[0].Alg)
	})

	t.Run("alg_defaults_by_kty", func(t *testing.T) {
		parsed, err := ParseClientJWKS(marshalJWKS(t,
			rsaJWK(t, rsaKey, "default-rsa", "", nil),
			ecJWK(t, ecKey, "default-ec", "", nil),
		))
		require.NoError(t, err)
		require.Len(t, parsed, 2)
		assert.Equal(t, "RS256", parsed[0].Alg)
		assert.Equal(t, "ES256", parsed[1].Alg)
	})

	t.Run("empty_keys_is_valid", func(t *testing.T) {
		parsed, err := ParseClientJWKS([]byte(`{"keys":[]}`))
		require.NoError(t, err)
		assert.Empty(t, parsed)
	})

	t.Run("use_sig_and_key_ops_verify_allowed", func(t *testing.T) {
		parsed, err := ParseClientJWKS(marshalJWKS(t,
			rsaJWK(t, rsaKey, "rsa-use", "RS256", map[string]any{
				"use":     "sig",
				"key_ops": []string{"verify"},
			}),
		))
		require.NoError(t, err)
		require.Len(t, parsed, 1)
	})
}

func TestParseClientJWKS_RejectsPrivateAndSymmetricMaterial(t *testing.T) {
	rsaKey := testRSA2048(t)

	t.Run("rsa_private_exponent", func(t *testing.T) {
		_, err := ParseClientJWKS(marshalJWKS(t, map[string]any{
			"kty": "RSA",
			"kid": "leaked",
			"n":   base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.E)).Bytes()),
			"d":   base64.RawURLEncoding.EncodeToString(rsaKey.D.Bytes()),
		}))
		assertJWKInvalid(t, err)
	})

	t.Run("rsa_private_prime", func(t *testing.T) {
		_, err := ParseClientJWKS(marshalJWKS(t, map[string]any{
			"kty": "RSA",
			"kid": "leaked-prime",
			"n":   base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.E)).Bytes()),
			"p":   base64.RawURLEncoding.EncodeToString(rsaKey.Primes[0].Bytes()),
		}))
		assertJWKInvalid(t, err)
	})

	t.Run("symmetric_oct_key", func(t *testing.T) {
		_, err := ParseClientJWKS(marshalJWKS(t, map[string]any{
			"kty": "oct",
			"kid": "shared-secret",
			"k":   base64.RawURLEncoding.EncodeToString([]byte("not-a-public-key")),
		}))
		assertJWKInvalid(t, err)
	})
}

func TestParseClientJWKS_RejectsUnsupportedAlgorithms(t *testing.T) {
	rsaKey := testRSA2048(t)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	cases := map[string]map[string]any{
		"none":              {"kty": "RSA", "kid": "none-key", "n": base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.E)).Bytes()), "alg": "none"},
		"hs256":             rsaJWK(t, rsaKey, "hs-key", "HS256", nil),
		"rs384":             rsaJWK(t, rsaKey, "rs384-key", "RS384", nil),
		"es512":             ecJWK(t, ecKey, "es512-key", "ES512", nil),
		"alg_for_other_kty": rsaJWK(t, rsaKey, "rsa-es256", "ES256", nil),
	}
	for name, key := range cases {
		key := key
		t.Run(name, func(t *testing.T) {
			_, err := ParseClientJWKS(marshalJWKS(t, key))
			assertJWKInvalid(t, err)
		})
	}
}

func TestParseClientJWKS_RejectsWeakOrMalformedKeys(t *testing.T) {
	weak, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)

	t.Run("rsa_below_2048", func(t *testing.T) {
		_, err := ParseClientJWKS(marshalJWKS(t, rsaJWK(t, weak, "weak-rsa", "", nil)))
		assertJWKInvalid(t, err)
	})

	t.Run("ec_curve_p384", func(t *testing.T) {
		key, genErr := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		require.NoError(t, genErr)
		_, err := ParseClientJWKS(marshalJWKS(t, map[string]any{
			"kty": "EC",
			"kid": "p384",
			"crv": "P-384",
			"alg": "ES256",
			"x":   base64.RawURLEncoding.EncodeToString(key.X.Bytes()),
			"y":   base64.RawURLEncoding.EncodeToString(key.Y.Bytes()),
		}))
		assertJWKInvalid(t, err)
	})

	t.Run("ec_point_not_on_curve", func(t *testing.T) {
		_, err := ParseClientJWKS(marshalJWKS(t, map[string]any{
			"kty": "EC",
			"kid": "off-curve",
			"crv": "P-256",
			"alg": "ES256",
			"x":   base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("x", 32))),
			"y":   base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("y", 32))),
		}))
		assertJWKInvalid(t, err)
	})

	t.Run("missing_kid", func(t *testing.T) {
		key := rsaJWK(t, testRSA2048(t), "", "RS256", nil)
		delete(key, "kid")
		_, err := ParseClientJWKS(marshalJWKS(t, key))
		assertJWKInvalid(t, err)
	})

	t.Run("kid_too_long", func(t *testing.T) {
		_, err := ParseClientJWKS(marshalJWKS(t, rsaJWK(t, testRSA2048(t), strings.Repeat("k", ClientJWKKidMaxLength+1), "RS256", nil)))
		assertJWKInvalid(t, err)
	})

	t.Run("duplicate_kid", func(t *testing.T) {
		_, err := ParseClientJWKS(marshalJWKS(t,
			rsaJWK(t, testRSA2048(t), "dupe", "RS256", nil),
			rsaJWK(t, testRSA2048(t), "dupe", "RS256", nil),
		))
		assertJWKInvalid(t, err)
	})

	t.Run("use_not_sig", func(t *testing.T) {
		_, err := ParseClientJWKS(marshalJWKS(t, rsaJWK(t, testRSA2048(t), "enc-key", "RS256", map[string]any{"use": "enc"})))
		assertJWKInvalid(t, err)
	})

	t.Run("key_ops_sign", func(t *testing.T) {
		_, err := ParseClientJWKS(marshalJWKS(t, rsaJWK(t, testRSA2048(t), "sign-key", "RS256", map[string]any{"key_ops": []string{"sign"}})))
		assertJWKInvalid(t, err)
	})

	t.Run("key_ops_without_verify", func(t *testing.T) {
		_, err := ParseClientJWKS(marshalJWKS(t, rsaJWK(t, testRSA2048(t), "wrap-key", "RS256", map[string]any{"key_ops": []string{"wrapKey"}})))
		assertJWKInvalid(t, err)
	})
}

func TestParseClientJWKS_RejectsDocumentLevelFailures(t *testing.T) {
	cases := map[string][]byte{
		"empty":              nil,
		"not_json":           []byte(`not json`),
		"missing_keys":       []byte(`{"notKeys":[]}`),
		"keys_not_array":     []byte(`{"keys":{}}`),
		"oversized_document": []byte(`{"keys":[],"padding":"` + strings.Repeat("p", ClientJWKMaxDocumentBytes) + `"}`),
	}
	for name, raw := range cases {
		raw := raw
		t.Run(name, func(t *testing.T) {
			_, err := ParseClientJWKS(raw)
			assertJWKInvalid(t, err)
		})
	}

	t.Run("too_many_keys", func(t *testing.T) {
		keys := make([]any, 0, ClientJWKMaxKeys+1)
		for i := 0; i <= ClientJWKMaxKeys; i++ {
			keys = append(keys, rsaJWK(t, testRSA2048(t), "kid-"+big.NewInt(int64(i)).String(), "RS256", nil))
		}
		_, err := ParseClientJWKS(marshalJWKS(t, keys...))
		assertJWKInvalid(t, err)
	})
}
