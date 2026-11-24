package wellknown

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/sm3"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appkeys "github.com/guliangiam/guliangiam/internal/app/keys"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domainkeys "github.com/guliangiam/guliangiam/internal/domain/keys"
	keysmock "github.com/guliangiam/guliangiam/internal/domain/keys/mock"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	rpKidPrefixRSA = platformjwks.KidPrefixRSA
	rpKidPrefixGM  = platformjwks.KidPrefixGM
)

var (
	errRPMalformedToken       = errors.New("rp: malformed token")
	errRPUnsupportedAlgorithm = errors.New("rp: unsupported signing algorithm")
	errRPKidAlgorithmMismatch = errors.New("rp: kid does not match signing algorithm")
	errRPUnknownKid           = errors.New("rp: unknown kid")
	errRPSignatureInvalid     = errors.New("rp: signature verification failed")
)

// rpJWK is a key as a relying party sees it: decoded from the JWKS JSON only.
// If a field is missing on the wire, it is missing for the RP as well.
type rpJWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type rpTokenHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

type rpDecodedToken struct {
	Header       rpTokenHeader
	SigningInput string
	Signature    []byte
}

// rpJWKSProvider adapts the platform Manager to the view the handler consumes.
// The production adapter is a field-for-field copy of this conversion
// (infra/jwt.JWKSProviderAdapter), which a transport test must not import:
// depguard keeps transport out of infra even for test files.
type rpJWKSProvider struct{ manager *platformjwks.Manager }

func (p rpJWKSProvider) GetJWKS() (*appkeys.JWKS, error) {
	set, err := p.manager.GetJWKS()
	if err != nil {
		return nil, err
	}
	return mapRPKeys(set), nil
}

func (p rpJWKSProvider) GetGMJWKS() (*appkeys.JWKS, error) {
	set, err := p.manager.GetGMJWKS()
	if err != nil {
		return nil, err
	}
	return mapRPKeys(set), nil
}

func mapRPKeys(set *platformjwks.JWKS) *appkeys.JWKS {
	keys := make([]appkeys.JWK, len(set.Keys))
	for i, key := range set.Keys {
		keys[i] = appkeys.JWK{
			Kty: key.Kty, Use: key.Use, Kid: key.Kid, Alg: key.Alg,
			N: key.N, E: key.E, Crv: key.Crv, X: key.X, Y: key.Y,
		}
	}
	return &appkeys.JWKS{Keys: keys}
}

type rpFixture struct {
	manager *platformjwks.Manager
	handler Handler
	// keys is what a dual-family RP holds after fetching both documents:
	// standardKeys from jwks_uri and gmKeys from gm_jwks_uri.
	keys         []rpJWK
	standardKeys []rpJWK
	gmKeys       []rpJWK
	rsaKey       rpJWK
	gmKey        rpJWK
}

func newRPFixture(t *testing.T) rpFixture {
	t.Helper()

	suffixes := []string{"rp-rsa", "rp-gm"}
	var next int
	manager := platformjwks.NewManager(2048, func() (string, error) {
		if next >= len(suffixes) {
			return "", errors.New("rp fixture: unexpected extra key generation")
		}
		suffix := suffixes[next]
		next++
		return suffix, nil
	})

	handler := Handler{
		JWKSManager: rpJWKSProvider{manager: manager},
		IssuerURL:   "https://issuer.rp-test.example",
	}
	standardKeys := fetchPublishedJWKS(t, handler, "/.well-known/jwks.json")
	gmKeys := fetchPublishedJWKS(t, handler, "/.well-known/jwks-gm.json")
	keys := append(append([]rpJWK(nil), standardKeys...), gmKeys...)
	rsaKey, gmKey := splitAlgorithmKeys(t, keys)
	return rpFixture{
		manager: manager, handler: handler,
		keys: keys, standardKeys: standardKeys, gmKeys: gmKeys,
		rsaKey: rsaKey, gmKey: gmKey,
	}
}

func fetchPublishedJWKS(t *testing.T, handler Handler, path string) []rpJWK {
	t.Helper()

	router := testRouter()
	RegisterChiRoutes(router, handler)
	resp := doRequest(t, router, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	var set struct {
		Keys []rpJWK `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(body, &set))
	require.NotEmpty(t, set.Keys)
	return set.Keys
}

func splitAlgorithmKeys(t *testing.T, keys []rpJWK) (rpJWK, rpJWK) {
	t.Helper()

	var rsaKey, gmKey rpJWK
	var rsaFound, gmFound bool
	for _, key := range keys {
		switch {
		case strings.HasPrefix(key.Kid, rpKidPrefixRSA):
			require.False(t, rsaFound, "JWKS publishes more than one rsa_ key")
			rsaKey, rsaFound = key, true
		case strings.HasPrefix(key.Kid, rpKidPrefixGM):
			require.False(t, gmFound, "JWKS publishes more than one gm_ key")
			gmKey, gmFound = key, true
		}
	}
	require.True(t, rsaFound, "JWKS is missing an rsa_ key: %+v", keys)
	require.True(t, gmFound, "JWKS is missing a gm_ key: %+v", keys)
	return rsaKey, gmKey
}

// rpVerify mirrors a relying party end to end: it decodes the protected header,
// selects the key from the published JWKS by kid, requires the kid prefix and
// the JWK `alg` to match the header algorithm, and only then verifies with
// independent SM2/RS256 primitives. There is no fallback key: an unknown kid is
// a hard failure, never "use the only key we have".
func rpVerify(token string, keys []rpJWK, allowSM2 bool) error {
	decoded, err := rpDecodeToken(token)
	if err != nil {
		return err
	}

	switch decoded.Header.Alg {
	case platformjwks.SigningAlgorithmRS256:
	case platformjwks.SigningAlgorithmSM2SM3:
		if !allowSM2 {
			return errRPUnsupportedAlgorithm
		}
	default:
		return errRPUnsupportedAlgorithm
	}

	key, err := rpSelectKey(keys, decoded.Header)
	if err != nil {
		return err
	}
	if key.Alg != decoded.Header.Alg {
		return errRPKidAlgorithmMismatch
	}

	switch decoded.Header.Alg {
	case platformjwks.SigningAlgorithmRS256:
		pub, err := key.rsaPublicKey()
		if err != nil {
			return err
		}
		hashed := sha256.Sum256([]byte(decoded.SigningInput))
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hashed[:], decoded.Signature); err != nil {
			return errRPSignatureInvalid
		}
	case platformjwks.SigningAlgorithmSM2SM3:
		pub, err := key.sm2PublicKey()
		if err != nil {
			return err
		}
		digest := sm3.Sum([]byte(decoded.SigningInput))
		if !sm2.VerifyASN1(pub, digest[:], decoded.Signature) {
			return errRPSignatureInvalid
		}
	}
	return nil
}

func rpSelectKey(keys []rpJWK, header rpTokenHeader) (rpJWK, error) {
	prefix := rpKidPrefixRSA
	if header.Alg == platformjwks.SigningAlgorithmSM2SM3 {
		prefix = rpKidPrefixGM
	}
	if !strings.HasPrefix(header.Kid, prefix) {
		return rpJWK{}, errRPKidAlgorithmMismatch
	}
	for _, key := range keys {
		if key.Kid == header.Kid {
			return key, nil
		}
	}
	return rpJWK{}, errRPUnknownKid
}

func rpDecodeToken(token string) (rpDecodedToken, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return rpDecodedToken{}, errRPMalformedToken
	}

	rawHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return rpDecodedToken{}, errRPMalformedToken
	}
	var header rpTokenHeader
	if err := json.Unmarshal(rawHeader, &header); err != nil {
		return rpDecodedToken{}, errRPMalformedToken
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return rpDecodedToken{}, errRPMalformedToken
	}
	return rpDecodedToken{
		Header:       header,
		SigningInput: parts[0] + "." + parts[1],
		Signature:    signature,
	}, nil
}

func (k rpJWK) rsaPublicKey() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, errRPMalformedToken
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, errRPMalformedToken
	}
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() || e.Int64() <= 0 {
		return nil, errRPMalformedToken
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(e.Int64())}, nil
}

func (k rpJWK) sm2PublicKey() (*ecdsa.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, errRPMalformedToken
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, errRPMalformedToken
	}
	// A strict RP checks the octet-string length before constructing the point:
	// RFC 7518 §6.2.1.2 requires the full curve size, so a short coordinate is
	// rejected rather than silently accepted by big.Int.SetBytes.
	if len(xBytes) != rpSM2CoordinateBytes || len(yBytes) != rpSM2CoordinateBytes {
		return nil, errRPMalformedToken
	}
	return &ecdsa.PublicKey{
		Curve: sm2.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}

// mintRPTestToken plays the issuer: it builds the exact signing input a compact
// JWT carries and signs it with the platform key the fixture generated.
func mintRPTestToken(t *testing.T, manager *platformjwks.Manager, alg, kid string, claims map[string]any) string {
	t.Helper()

	header := map[string]string{"alg": alg, "typ": "JWT", "kid": kid}
	headerJSON, err := json.Marshal(header)
	require.NoError(t, err)
	payloadJSON, err := json.Marshal(claims)
	require.NoError(t, err)
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)

	var signature []byte
	switch alg {
	case platformjwks.SigningAlgorithmRS256:
		kp, keyErr := manager.GetKeyPair()
		require.NoError(t, keyErr)
		hashed := sha256.Sum256([]byte(signingInput))
		signature, err = rsa.SignPKCS1v15(rand.Reader, kp.Key, crypto.SHA256, hashed[:])
		require.NoError(t, err)
	case platformjwks.SigningAlgorithmSM2SM3:
		kp, keyErr := manager.GetGMKeyPair()
		require.NoError(t, keyErr)
		signature, err = platformcrypto.NewCryptoSuite().SM2Sign(kp.PrivateKey, platformcrypto.SM3Sum([]byte(signingInput)))
		require.NoError(t, err)
	default:
		t.Fatalf("rp fixture: unsupported signing algorithm %q", alg)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

// tamperSignature flips one bit of the final signature byte: the token still
// parses, so only a real signature check can reject it.
func tamperSignature(t *testing.T, token string) string {
	t.Helper()

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	raw[len(raw)-1] ^= 0x01
	return parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(raw)
}

func publishedKids(keys []rpJWK) []string {
	kids := make([]string, len(keys))
	for i, key := range keys {
		kids[i] = key.Kid
	}
	return kids
}

func TestPublishedJWKSContainsRSAAndSM2Keys(t *testing.T) {
	fixture := newRPFixture(t)

	// The families are published on separate endpoints: the standard document
	// carries only rsa_ keys so strict JOSE parsers never see the SM2 curve,
	// and the GM document carries only gm_ keys.
	for _, key := range fixture.standardKeys {
		assert.Truef(t, strings.HasPrefix(key.Kid, rpKidPrefixRSA),
			"standard jwks.json publishes non-rsa_ kid %q", key.Kid)
		assert.NotEqual(t, "SM2", key.Crv)
		assert.NotEqual(t, platformjwks.SigningAlgorithmSM2SM3, key.Alg)
	}
	for _, key := range fixture.gmKeys {
		assert.Truef(t, strings.HasPrefix(key.Kid, rpKidPrefixGM),
			"jwks-gm.json publishes non-gm_ kid %q", key.Kid)
	}
	t.Logf("httptest GET /.well-known/jwks.json -> kids=%v; /.well-known/jwks-gm.json -> kids=%v",
		publishedKids(fixture.standardKeys), publishedKids(fixture.gmKeys))

	assert.Equal(t, "RSA", fixture.rsaKey.Kty)
	assert.Equal(t, "sig", fixture.rsaKey.Use)
	assert.Equal(t, platformjwks.SigningAlgorithmRS256, fixture.rsaKey.Alg)
	assert.NotEmpty(t, fixture.rsaKey.N)
	assert.NotEmpty(t, fixture.rsaKey.E)
	assert.Empty(t, fixture.rsaKey.Crv)
	assert.Empty(t, fixture.rsaKey.X)
	assert.Empty(t, fixture.rsaKey.Y)

	assert.Equal(t, "EC", fixture.gmKey.Kty)
	assert.Equal(t, "sig", fixture.gmKey.Use)
	assert.Equal(t, platformjwks.SigningAlgorithmSM2SM3, fixture.gmKey.Alg)
	assert.Equal(t, "SM2", fixture.gmKey.Crv)
	assert.NotEmpty(t, fixture.gmKey.X)
	assert.NotEmpty(t, fixture.gmKey.Y)
	assert.Empty(t, fixture.gmKey.N)
	assert.Empty(t, fixture.gmKey.E)

	// An RP caches keys by kid; a later fetch must repeat the same ids on both
	// endpoints.
	assert.Equal(t, publishedKids(fixture.standardKeys), publishedKids(fetchPublishedJWKS(t, fixture.handler, "/.well-known/jwks.json")))
	assert.Equal(t, publishedKids(fixture.gmKeys), publishedKids(fetchPublishedJWKS(t, fixture.handler, "/.well-known/jwks-gm.json")))
}

func TestRelyingPartyVerifiesSM2TokenFromPublishedJWKS(t *testing.T) {
	fixture := newRPFixture(t)

	token := mintRPTestToken(t, fixture.manager, platformjwks.SigningAlgorithmSM2SM3, fixture.gmKey.Kid,
		map[string]any{"sub": "user-1", "iss": "https://issuer.rp-test.example"})
	// The GM document alone is sufficient for a GM-capable RP.
	require.NoError(t, rpVerify(token, fixture.gmKeys, true))
	require.NoError(t, rpVerify(token, fixture.keys, true))
}

func TestRelyingPartyVerifiesRS256TokenFromPublishedJWKS(t *testing.T) {
	fixture := newRPFixture(t)

	token := mintRPTestToken(t, fixture.manager, platformjwks.SigningAlgorithmRS256, fixture.rsaKey.Kid,
		map[string]any{"sub": "user-1", "iss": "https://issuer.rp-test.example"})
	// The standard document alone is sufficient for an RS256 RP.
	require.NoError(t, rpVerify(token, fixture.standardKeys, true))
	require.NoError(t, rpVerify(token, fixture.keys, true))
}

// TestRS256OnlyRelyingPartyIgnoresGMKey pins the migration path: an RP that
// cannot do SM2 keeps working off the standard document, which now carries no
// gm_ key at all — the SM2 curve can no longer break its key-set parsing.
func TestRS256OnlyRelyingPartyIgnoresGMKey(t *testing.T) {
	fixture := newRPFixture(t)

	rsaOnly := make([]rpJWK, 0, len(fixture.standardKeys))
	for _, key := range fixture.standardKeys {
		if key.Alg == platformjwks.SigningAlgorithmRS256 {
			rsaOnly = append(rsaOnly, key)
		}
	}
	require.Len(t, rsaOnly, 1)
	require.Len(t, fixture.standardKeys, len(rsaOnly), "standard JWKS must not carry any GM key")

	token := mintRPTestToken(t, fixture.manager, platformjwks.SigningAlgorithmRS256, fixture.rsaKey.Kid,
		map[string]any{"sub": "user-1"})
	require.NoError(t, rpVerify(token, rsaOnly, false))
	require.NoError(t, rpVerify(token, fixture.standardKeys, false), "an RSA-only RP must parse and verify off the standard document alone")

	sm2Token := mintRPTestToken(t, fixture.manager, platformjwks.SigningAlgorithmSM2SM3, fixture.gmKey.Kid,
		map[string]any{"sub": "user-1"})
	require.ErrorIs(t, rpVerify(sm2Token, fixture.standardKeys, false), errRPUnsupportedAlgorithm)
	// Even a GM-capable verifier finds no gm_ key in the standard document; it
	// must fetch gm_jwks_uri instead.
	require.ErrorIs(t, rpVerify(sm2Token, fixture.standardKeys, true), errRPUnknownKid)
	require.NoError(t, rpVerify(sm2Token, fixture.gmKeys, true))
}

func TestRelyingPartyRejectsTamperedSignature(t *testing.T) {
	fixture := newRPFixture(t)

	cases := []struct {
		name string
		alg  string
		kid  string
	}{
		{"rs256", platformjwks.SigningAlgorithmRS256, fixture.rsaKey.Kid},
		{"sm2-sm3", platformjwks.SigningAlgorithmSM2SM3, fixture.gmKey.Kid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := mintRPTestToken(t, fixture.manager, tc.alg, tc.kid, map[string]any{"sub": "user-1"})
			require.NoError(t, rpVerify(token, fixture.keys, true))
			tampered := tamperSignature(t, token)
			err := rpVerify(tampered, fixture.keys, true)
			t.Logf("%s: one-bit signature tamper -> %v", tc.name, err)
			require.ErrorIs(t, err, errRPSignatureInvalid)
		})
	}
}

// TestRelyingPartyRejectsCrossAlgorithmKey covers the misuse AC-CRYPTO-002
// names: a gm_ key must not verify an RSA signature (and vice versa), and a
// kid whose prefix disagrees with the header alg is rejected before any crypto.
func TestRelyingPartyRejectsCrossAlgorithmKey(t *testing.T) {
	fixture := newRPFixture(t)

	rsaToken := mintRPTestToken(t, fixture.manager, platformjwks.SigningAlgorithmRS256, fixture.rsaKey.Kid,
		map[string]any{"sub": "user-1"})
	rsaDecoded, err := rpDecodeToken(rsaToken)
	require.NoError(t, err)
	gmPub, err := fixture.gmKey.sm2PublicKey()
	require.NoError(t, err)

	gmDigest := sm3.Sum([]byte(rsaDecoded.SigningInput))
	crossOK := sm2.VerifyASN1(gmPub, gmDigest[:], rsaDecoded.Signature)
	t.Logf("gm_ public key vs RS256 signature -> verified=%v (want false)", crossOK)
	require.False(t, crossOK, "the gm_ public key verified an RS256 signature")

	sm2Token := mintRPTestToken(t, fixture.manager, platformjwks.SigningAlgorithmSM2SM3, fixture.gmKey.Kid,
		map[string]any{"sub": "user-1"})
	sm2Decoded, err := rpDecodeToken(sm2Token)
	require.NoError(t, err)
	rsaPub, err := fixture.rsaKey.rsaPublicKey()
	require.NoError(t, err)
	rsaHashed := sha256.Sum256([]byte(sm2Decoded.SigningInput))
	rsaErr := rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, rsaHashed[:], sm2Decoded.Signature)
	t.Logf("rsa_ public key vs SM2-SM3 signature -> %v (want error)", rsaErr)
	require.Error(t, rsaErr, "the rsa_ public key verified an SM2-SM3 signature")

	// A header that claims RS256 but names a gm_ kid is a cross-algorithm kid:
	// the RP rejects it on the prefix, before touching either primitive.
	crossKid := mintRPTestToken(t, fixture.manager, platformjwks.SigningAlgorithmRS256, fixture.gmKey.Kid,
		map[string]any{"sub": "user-1"})
	t.Logf("RS256 token with gm_ kid -> %v", rpVerify(crossKid, fixture.keys, true))
	require.ErrorIs(t, rpVerify(crossKid, fixture.keys, true), errRPKidAlgorithmMismatch)

	// The same rule holds inside the platform lookup used by our own issuer.
	_, err = fixture.manager.GetKeyPairByID(fixture.gmKey.Kid)
	require.Error(t, err, "rsa lookup accepted a gm_ kid")
	_, err = fixture.manager.GetGMKeyPairByID(fixture.rsaKey.Kid)
	require.Error(t, err, "gm lookup accepted an rsa_ kid")
}

// TestRelyingPartyRejectsUnknownKidWithoutFallback is the regression for silent
// fallback: the signature below is valid under the current gm_ key, so a
// verifier that ignored the kid would accept it. The RP must reject it because
// the kid it names is not published.
func TestRelyingPartyRejectsUnknownKidWithoutFallback(t *testing.T) {
	fixture := newRPFixture(t)

	unknownKid := rpKidPrefixGM + "not-published"
	token := mintRPTestToken(t, fixture.manager, platformjwks.SigningAlgorithmSM2SM3, unknownKid,
		map[string]any{"sub": "user-1"})
	decoded, err := rpDecodeToken(token)
	require.NoError(t, err)
	gmPub, err := fixture.gmKey.sm2PublicKey()
	require.NoError(t, err)

	digest := sm3.Sum([]byte(decoded.SigningInput))
	validUnderCurrentKey := sm2.VerifyASN1(gmPub, digest[:], decoded.Signature)
	require.True(t, validUnderCurrentKey,
		"fixture is invalid: the unknown-kid token must be valid under the current gm_ key")
	unknownErr := rpVerify(token, fixture.keys, true)
	t.Logf("unknown kid %q (signature valid under current gm_ key) -> %v", unknownKid, unknownErr)
	require.ErrorIs(t, unknownErr, errRPUnknownKid)

	_, err = fixture.manager.GetGMKeyPairByID(unknownKid)
	require.Error(t, err, "platform gm lookup silently resolved an unknown kid")
	_, err = fixture.manager.GetKeyPairByID(rpKidPrefixRSA + "not-published")
	require.Error(t, err, "platform rsa lookup silently resolved an unknown kid")
}

// TestApplicationJWKSKeepsPlatformSigningKeys pins AC-CRYPTO-002 for the
// application-scoped jwks_uri: the issuer signs with the platform key pair, so
// an application document that only carried its key record left an RP without
// the platform rsa_ key an RS256 token names. Like the global standard
// endpoint, the application document stays free of the SM2 curve; SM2-SM3
// tokens are verified against the dedicated GM endpoint.
func TestApplicationJWKSKeepsPlatformSigningKeys(t *testing.T) {
	manager := platformjwks.NewManager(2048, nil)
	appPublicKeyPEM := testRSAPublicKeyPEM(t)

	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockAuthUoW(ctrl)
	repo := keysmock.NewMockRepository(ctrl)
	uow.EXPECT().Keys().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainkeys.Key{{
		ID:             "app-key-1",
		OrganizationID: "org-1",
		Name:           "app-signing-app1",
		KeyType:        "rsa",
		PublicKey:      appPublicKeyPEM,
		Algorithm:      platformjwks.SigningAlgorithmRS256,
		IsEnabled:      true,
	}}, int64(1), nil).AnyTimes()

	handler := Handler{
		JWKSManager:        rpJWKSProvider{manager: manager},
		OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{application: &appauth.OIDCApplication{Slug: "app1", OrganizationID: "org-1"}},
		KeyService:         &appkeys.Service{UOW: uow},
	}
	keys := fetchPublishedJWKS(t, handler, "/.well-known/app1/jwks.json")
	require.Len(t, keys, 2, "application JWKS = %+v", publishedKids(keys))

	wantAppKid := platformjwks.KidPrefixRSA + "app-key-1"
	appKeyIndex := -1
	for i := range keys {
		if keys[i].Kid == wantAppKid {
			appKeyIndex = i
		}
	}
	require.NotEqual(t, -1, appKeyIndex, "application entry was dropped: %+v", publishedKids(keys))
	assert.Equal(t, "RSA", keys[appKeyIndex].Kty)

	// The application document carries no GM key: the SM2 curve must not reach
	// strict JOSE parsers through any standard jwks_uri.
	for _, key := range keys {
		require.NotEqual(t, "SM2", key.Crv, "application JWKS leaked a GM key: %+v", key)
		require.NotEqual(t, platformjwks.SigningAlgorithmSM2SM3, key.Alg)
		require.False(t, strings.HasPrefix(key.Kid, rpKidPrefixGM))
	}

	// The platform entries keep their platform kid in both documents: the same
	// public key must never be published under two different kids.
	platformKeys := append(append([]rpJWK(nil), keys[:appKeyIndex]...), keys[appKeyIndex+1:]...)
	globalKeys := fetchPublishedJWKS(t, handler, "/.well-known/jwks.json")
	assert.Equal(t, publishedKids(globalKeys), publishedKids(platformKeys),
		"platform keys changed kid between the global and application JWKS")
	require.Len(t, platformKeys, 1)
	rsaKey := platformKeys[0]
	require.True(t, strings.HasPrefix(rsaKey.Kid, rpKidPrefixRSA))

	rsaToken := mintRPTestToken(t, manager, platformjwks.SigningAlgorithmRS256, rsaKey.Kid,
		map[string]any{"sub": "user-1"})
	require.NoError(t, rpVerify(rsaToken, keys, true))

	// An SM2-SM3 token names a gm_ kid that only the GM endpoint publishes.
	gmKeys := fetchPublishedJWKS(t, handler, "/.well-known/jwks-gm.json")
	gmKey := gmKeys[0]
	require.True(t, strings.HasPrefix(gmKey.Kid, rpKidPrefixGM))
	sm2Token := mintRPTestToken(t, manager, platformjwks.SigningAlgorithmSM2SM3, gmKey.Kid,
		map[string]any{"sub": "user-1"})
	require.ErrorIs(t, rpVerify(sm2Token, keys, true), errRPUnknownKid,
		"the application standard document must not publish the gm_ key")
	require.NoError(t, rpVerify(sm2Token, gmKeys, true),
		"the GM endpoint cannot verify an SM2-SM3 token")
}

// rpSM2CoordinateBytes is the SM2 field element size. A strict RP requires an
// EC JWK coordinate to decode to exactly this length (RFC 7518 §6.2.1.2).
const rpSM2CoordinateBytes = 32

// leadingZeroSM2PrivateKeyPEM returns SM2 key material whose public point has a
// coordinate that big.Int.Bytes() would shorten by dropping a high zero byte.
// It needs ~128 attempts on average; the cap keeps a bad RNG from hanging the
// test.
func leadingZeroSM2PrivateKeyPEM(t *testing.T) string {
	t.Helper()

	for attempt := 1; attempt <= 2000; attempt++ {
		pemData, err := cryptotest.GenerateSM2PrivateKeyPEM()
		require.NoError(t, err)
		key, err := platformcrypto.ParseSM2PrivateKeyPEM(pemData)
		require.NoError(t, err)
		x, y, ok := key.Public().Coordinates()
		require.True(t, ok)
		if len(x.Bytes()) < rpSM2CoordinateBytes || len(y.Bytes()) < rpSM2CoordinateBytes {
			t.Logf("leading-zero SM2 key found after %d attempt(s): x=%d bytes, y=%d bytes",
				attempt, len(x.Bytes()), len(y.Bytes()))
			return pemData
		}
	}
	t.Fatal("no SM2 key with a leading-zero coordinate in 2000 attempts")
	return ""
}

// TestRelyingPartyVerifiesSM2TokenWithLeadingZeroCoordinates is the GM R3
// regression end to end: a strict RP rejects a short coordinate before any
// crypto runs, so the published JWKS must always carry full-length x/y.
func TestRelyingPartyVerifiesSM2TokenWithLeadingZeroCoordinates(t *testing.T) {
	pemData := leadingZeroSM2PrivateKeyPEM(t)

	ringJSON, err := json.Marshal(map[string]any{
		"keys": []map[string]any{{
			"kid":             rpKidPrefixGM + "leading-zero",
			"algorithm":       platformjwks.SigningAlgorithmSM2SM3,
			"private_key_pem": pemData,
			"created_at":      time.Now().UTC(),
		}},
	})
	require.NoError(t, err)
	ring, err := platformjwks.ParseKeyRing(ringJSON)
	require.NoError(t, err)
	manager, err := platformjwks.NewManagerWithKeyRing(ring)
	require.NoError(t, err)

	handler := Handler{
		JWKSManager: rpJWKSProvider{manager: manager},
		IssuerURL:   "https://issuer.rp-leading-zero.example",
	}
	keys := fetchPublishedJWKS(t, handler, "/.well-known/jwks-gm.json")
	require.Len(t, keys, 1)
	gmKey := keys[0]
	require.True(t, strings.HasPrefix(gmKey.Kid, rpKidPrefixGM))
	require.Equal(t, "SM2", gmKey.Crv)

	xBytes, err := base64.RawURLEncoding.DecodeString(gmKey.X)
	require.NoError(t, err)
	yBytes, err := base64.RawURLEncoding.DecodeString(gmKey.Y)
	require.NoError(t, err)
	require.Len(t, xBytes, rpSM2CoordinateBytes, "strict RP requires the full SM2 coordinate length")
	require.Len(t, yBytes, rpSM2CoordinateBytes, "strict RP requires the full SM2 coordinate length")

	token := mintRPTestToken(t, manager, platformjwks.SigningAlgorithmSM2SM3, gmKey.Kid,
		map[string]any{"sub": "user-1"})
	require.NoError(t, rpVerify(token, keys, true))
}

func testRSAPublicKeyPEM(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}
