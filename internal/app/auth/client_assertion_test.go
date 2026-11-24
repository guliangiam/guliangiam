package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applications "github.com/guliangiam/guliangiam/internal/domain/applications"
	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 本文件锁定 private_key_jwt 验证器的红线路由：kid 必须绑定本客户端、aud 必须
// 是本次 token 端点的绝对 URL、iss/sub 必须等于 client_id、exp 窗口受 5 分钟
// 上限约束、jti 不得重放；任何失败都返回同一个 401 文案，调用方无法从错误码或
// 文案区分“客户端不存在”和“密钥错误”。

const (
	testTokenEndpointURL = "https://iam.example.com/api/v1/auth/tokens/exchange"
	testAppJWKKid        = "client-key-1"
)

var (
	testAssertionRSAOnce sync.Once
	testAssertionRSA     *rsa.PrivateKey
	testAssertionRSAErr  error
)

func testAssertionRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	testAssertionRSAOnce.Do(func() {
		testAssertionRSA, testAssertionRSAErr = rsa.GenerateKey(rand.Reader, 2048)
	})
	require.NoError(t, testAssertionRSAErr)
	return testAssertionRSA
}

func rsaPublicJWKJSON(t *testing.T, key *rsa.PrivateKey, kid, alg string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"kty": "RSA",
		"kid": kid,
		"alg": alg,
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	})
	require.NoError(t, err)
	return string(raw)
}

func ecPublicJWKJSON(t *testing.T, key *ecdsa.PrivateKey, kid string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"kty": "EC",
		"kid": kid,
		"alg": "ES256",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(key.X.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(key.Y.Bytes()),
	})
	require.NoError(t, err)
	return string(raw)
}

// signClientAssertion 构造并签名一个 RFC 7523 客户端断言。
func signClientAssertion(t *testing.T, alg, kid string, claims map[string]any, signer crypto.Signer) string {
	t.Helper()

	header := map[string]any{"alg": alg, "typ": "JWT", "kid": kid}
	headerJSON, err := json.Marshal(header)
	require.NoError(t, err)
	claimsJSON, err := json.Marshal(claims)
	require.NoError(t, err)

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(signingInput))

	var signature []byte
	switch strings.ToUpper(alg) {
	case "RS256":
		rsaKey, ok := signer.(*rsa.PrivateKey)
		require.True(t, ok, "RS256 needs an RSA signer")
		signature, err = rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, digest[:])
		require.NoError(t, err)
	case "PS256":
		rsaKey, ok := signer.(*rsa.PrivateKey)
		require.True(t, ok, "PS256 needs an RSA signer")
		signature, err = rsa.SignPSS(rand.Reader, rsaKey, crypto.SHA256, digest[:],
			&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
		require.NoError(t, err)
	case "ES256":
		ecKey, ok := signer.(*ecdsa.PrivateKey)
		require.True(t, ok, "ES256 needs an EC signer")
		r, s, signErr := ecdsa.Sign(rand.Reader, ecKey, digest[:])
		require.NoError(t, signErr)
		signature = make([]byte, 64)
		r.FillBytes(signature[:32])
		s.FillBytes(signature[32:])
	case "HS256":
		// HS256 用公开 JWK 字节当 HMAC 密钥：验证器必须在验签前就拒绝该算法。
		mac := hmac.New(sha256.New, []byte("public-jwk-bytes"))
		mac.Write([]byte(signingInput))
		signature = mac.Sum(nil)
	case "NONE":
		signature = nil
	default:
		require.FailNow(t, "unsupported test algorithm "+alg)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func assertionClaims(clientID string, exp time.Time, overrides map[string]any) map[string]any {
	claims := map[string]any{
		"iss": clientID,
		"sub": clientID,
		"aud": testTokenEndpointURL,
		"exp": exp.Unix(),
		"iat": exp.Add(-time.Minute).Unix(),
		"jti": "jti-" + exp.Format(time.RFC3339Nano),
	}
	for key, value := range overrides {
		if value == nil {
			delete(claims, key)
			continue
		}
		claims[key] = value
	}
	return claims
}

func newAssertionService(t *testing.T) (Service, *loginTestAppRepository, *applications.Application, loginTestUnitOfWork) {
	t.Helper()
	app := &applications.Application{
		ID:                      testAppID,
		OrganizationID:          testOrgID,
		ClientID:                testAppClientID,
		TokenEndpointAuthMethod: applications.TokenEndpointAuthMethodPrivateKeyJWT,
	}
	repo := &loginTestAppRepository{
		apps:       map[string]*applications.Application{testAppID: app},
		byClientID: map[string]*applications.Application{testAppClientID: app},
	}
	uow := loginTestUnitOfWork{appRepo: repo}
	svc := newTestService(uow)
	// 每个用例一个独立 replay store，避免包级替身在用例之间串味。
	svc.ClientAssertionReplayStore = newMemoryAssertionReplayStore()
	fakeAppClientJWKs = map[string][]*applications.ClientJWK{}
	return svc, repo, app, uow
}

func registerClientJWK(t *testing.T, repo *loginTestAppRepository, organizationID, kid, alg, jwk string) {
	t.Helper()
	require.NoError(t, repo.ReplaceClientJWKs(context.Background(), organizationID, testAppID,
		[]*applications.ClientJWK{{
			ID:             "jwk-" + kid,
			ApplicationID:  testAppID,
			OrganizationID: organizationID,
			Kid:            kid,
			Alg:            alg,
			JWK:            jwk,
			Status:         applications.ClientJWKStatusActive,
		}}, time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)))
}

func assertClientAuthFailure(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	// 同一错误码 + 同一文案：调用方无法区分“客户端不存在”和“密钥错误”。
	assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)
	assert.Equal(t, 401, appErr.HTTPStatus())
	assert.Equal(t, clientAuthFailureMessage, appErr.Message)
}

func TestVerifyPrivateKeyJWTAssertion_AcceptsRS256PS256ES256(t *testing.T) {
	rsaKey := testAssertionRSAKey(t)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	cases := []struct {
		name string
		alg  string
		kid  string
		jwk  string
		key  crypto.Signer
	}{
		{"rs256", "RS256", "rsa-rs256", rsaPublicJWKJSON(t, rsaKey, "rsa-rs256", "RS256"), rsaKey},
		{"ps256", "PS256", "rsa-ps256", rsaPublicJWKJSON(t, rsaKey, "rsa-ps256", "PS256"), rsaKey},
		{"es256", "ES256", "ec-es256", ecPublicJWKJSON(t, ecKey, "ec-es256"), ecKey},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, app, uow := newAssertionService(t)
			registerClientJWK(t, repo, testOrgID, tc.kid, tc.alg, tc.jwk)

			now := svc.Clock.Now().UTC()
			assertion := signClientAssertion(t, tc.alg, tc.kid, assertionClaims(app.ClientID, now.Add(2*time.Minute), nil), tc.key)

			err := svc.verifyPrivateKeyJWTAssertion(context.Background(), uow, app, assertion, ClientAssertionTypeJWT, testTokenEndpointURL)
			require.NoError(t, err)
		})
	}
}

func TestVerifyPrivateKeyJWTAssertion_RejectsEveryFailureWithSame401(t *testing.T) {
	rsaKey := testAssertionRSAKey(t)
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	cases := []struct {
		name     string
		mutate   func(svc *Service, repo *loginTestAppRepository, app *applications.Application) (string, string)
		setupKey bool
	}{
		{
			name: "unknown_kid",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "RS256", "unknown-kid", assertionClaims(app.ClientID, now.Add(time.Minute), nil), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: true,
		},
		{
			name: "tampered_signature",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				assertion := signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), nil), otherKey)
				return assertion, ClientAssertionTypeJWT
			},
			setupKey: true,
		},
		{
			name: "wrong_audience",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), map[string]any{"aud": "https://evil.example.com/token"}), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: true,
		},
		{
			name: "iss_not_client_id",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), map[string]any{"iss": "someone-else"}), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: true,
		},
		{
			name: "sub_not_client_id",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), map[string]any{"sub": "someone-else"}), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: true,
		},
		{
			name: "expired",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(-time.Minute), nil), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: true,
		},
		{
			name: "exp_beyond_five_minutes",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(clientAssertionMaxLifetime+clientAssertionClockSkew+time.Minute), nil), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: true,
		},
		{
			name: "missing_jti",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), map[string]any{"jti": nil}), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: true,
		},
		{
			name: "wrong_assertion_type",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), nil), rsaKey), "urn:wrong"
			},
			setupKey: true,
		},
		{
			name: "alg_none",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "none", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), nil), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: true,
		},
		{
			name: "alg_hs256_never_accepted",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "HS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), nil), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: true,
		},
		{
			name: "no_registered_key",
			mutate: func(_ *Service, _ *loginTestAppRepository, app *applications.Application) (string, string) {
				return signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), nil), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: false,
		},
		{
			name: "key_belongs_to_other_tenant",
			mutate: func(_ *Service, repo *loginTestAppRepository, app *applications.Application) (string, string) {
				// 公钥登记在另一个 organization_id 下；本应用所属租户读不到该 key。
				registerClientJWK(t, repo, "other-org", testAppJWKKid, "RS256", rsaPublicJWKJSON(t, rsaKey, testAppJWKKid, "RS256"))
				return signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), nil), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: false,
		},
		{
			name: "registered_key_alg_mismatch",
			mutate: func(_ *Service, repo *loginTestAppRepository, app *applications.Application) (string, string) {
				registerClientJWK(t, repo, testOrgID, testAppJWKKid, "RS256", rsaPublicJWKJSON(t, rsaKey, testAppJWKKid, "RS256"))
				return signClientAssertion(t, "PS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), nil), rsaKey), ClientAssertionTypeJWT
			},
			setupKey: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, app, uow := newAssertionService(t)
			if tc.setupKey {
				registerClientJWK(t, repo, testOrgID, testAppJWKKid, "RS256", rsaPublicJWKJSON(t, testAssertionRSAKey(t), testAppJWKKid, "RS256"))
			}
			assertion, assertionType := tc.mutate(&svc, repo, app)
			err := svc.verifyPrivateKeyJWTAssertion(context.Background(), uow, app, assertion, assertionType, testTokenEndpointURL)
			assertClientAuthFailure(t, err)
		})
	}
}

func TestVerifyPrivateKeyJWTAssertion_RejectsReplayedJTI(t *testing.T) {
	svc, repo, app, uow := newAssertionService(t)
	registerClientJWK(t, repo, testOrgID, testAppJWKKid, "RS256", rsaPublicJWKJSON(t, testAssertionRSAKey(t), testAppJWKKid, "RS256"))

	now := svc.Clock.Now().UTC()
	assertion := signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(2*time.Minute), nil), testAssertionRSAKey(t))

	require.NoError(t, svc.verifyPrivateKeyJWTAssertion(context.Background(), uow, app, assertion, ClientAssertionTypeJWT, testTokenEndpointURL))
	err := svc.verifyPrivateKeyJWTAssertion(context.Background(), uow, app, assertion, ClientAssertionTypeJWT, testTokenEndpointURL)
	assertClientAuthFailure(t, err)
}

func TestVerifyPrivateKeyJWTAssertion_RejectsMalformedInputs(t *testing.T) {
	svc, repo, app, uow := newAssertionService(t)
	registerClientJWK(t, repo, testOrgID, testAppJWKKid, "RS256", rsaPublicJWKJSON(t, testAssertionRSAKey(t), testAppJWKKid, "RS256"))

	cases := map[string]struct {
		assertion  string
		audience   string
		assertType string
	}{
		"not_three_segments": {"abc", testTokenEndpointURL, ClientAssertionTypeJWT},
		"empty":              {"", testTokenEndpointURL, ClientAssertionTypeJWT},
		"oversized":          {strings.Repeat("x", maxClientAssertionBytes+1), testTokenEndpointURL, ClientAssertionTypeJWT},
		"audience_not_abs":   {signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, svc.Clock.Now().Add(time.Minute), nil), testAssertionRSAKey(t)), "/relative", ClientAssertionTypeJWT},
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := svc.verifyPrivateKeyJWTAssertion(context.Background(), uow, app, tc.assertion, tc.assertType, tc.audience)
			assertClientAuthFailure(t, err)
		})
	}
}

func TestVerifyPrivateKeyJWTAssertion_NilApplicationFails401(t *testing.T) {
	svc, _, _, uow := newAssertionService(t)
	err := svc.verifyPrivateKeyJWTAssertion(context.Background(), uow, nil, "a.b.c", ClientAssertionTypeJWT, testTokenEndpointURL)
	assertClientAuthFailure(t, err)
}

// staticJWKSProvider exposes one platform key pair. It stands in for the
// platform signer a client assertion must never be able to borrow.
type staticJWKSProvider struct{ keyPair *domainauth.KeyPair }

func (p staticJWKSProvider) GetKeyPair() (*domainauth.KeyPair, error) { return p.keyPair, nil }
func (p staticJWKSProvider) GetJWKS() (*domainauth.JWKS, error)       { return &domainauth.JWKS{}, nil }
func (p staticJWKSProvider) GetGMJWKS() (*domainauth.JWKS, error)     { return &domainauth.JWKS{}, nil }
func (p staticJWKSProvider) GetPublicKey() (*rsa.PublicKey, error) {
	if p.keyPair == nil || p.keyPair.Key == nil {
		return nil, nil
	}
	return &p.keyPair.Key.PublicKey, nil
}
func (p staticJWKSProvider) SignToken(map[string]any, string) (string, error) { return "", nil }
func (p staticJWKSProvider) SignTokenWithAlgorithm(map[string]any, string, string) (string, error) {
	return "", nil
}

// TestVerifyPrivateKeyJWTAssertion_NeverFallsBackToPlatformKey locks the
// removal of the old pseudo-implementation: an assertion signed by the platform
// key must be rejected unless the presented kid belongs to this client's
// app_client_jwks. The platform key pair is wired into the service on purpose,
// so the only reason the assertion fails is the missing client-owned kid.
func TestVerifyPrivateKeyJWTAssertion_NeverFallsBackToPlatformKey(t *testing.T) {
	platformKey := testAssertionRSAKey(t)

	cases := []struct {
		name               string
		registerClientJWKS bool
	}{
		{"client_has_no_jwks", false},
		{"client_jwks_kid_mismatch", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, app, uow := newAssertionService(t)
			svc.JWKSManager = staticJWKSProvider{keyPair: &domainauth.KeyPair{
				Kid:       "platform-key-1",
				Algorithm: "RS256",
				Key:       platformKey,
			}}
			if tc.registerClientJWKS {
				// The client owns the platform key material but under a different
				// kid; a kid-less fallback would still accept the assertion.
				registerClientJWK(t, repo, testOrgID, testAppJWKKid, "RS256", rsaPublicJWKJSON(t, platformKey, testAppJWKKid, "RS256"))
			}

			assertion := signClientAssertion(t, "RS256", "platform-key-1",
				assertionClaims(app.ClientID, svc.Clock.Now().UTC().Add(2*time.Minute), nil), platformKey)
			err := svc.verifyPrivateKeyJWTAssertion(context.Background(), uow, app, assertion, ClientAssertionTypeJWT, testTokenEndpointURL)
			assertClientAuthFailure(t, err)
		})
	}
}
