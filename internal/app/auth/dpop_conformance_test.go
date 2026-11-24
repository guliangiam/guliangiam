package auth

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	platformdpop "github.com/guliangiam/guliangiam/internal/platform/dpop"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin 设计报告 14.27 (RFC 9449) as implemented for. The
// defects were: the token-exchange path passed URL "" so `urlMatches` failed
// against every proof (a dpop_required application could never exchange a code),
// the algorithm whitelist only lived in an unassembled validator so an OKP proof
// could declare any `alg`, the proof result was discarded so `cnf.jkt` was never
// written, and the effective iat window was 5 minutes instead of ±60s.

// dpopTestSigner mints real DPoP proofs. The existing platform tests use dummy
// JWK material, so they never reach the signature check — which is exactly how
// the OKP branch kept accepting arbitrary algorithms.
type dpopTestSigner struct {
	rsaKey *rsa.PrivateKey
	rsaJWK json.RawMessage
	edKey  ed25519.PrivateKey
	edJWK  json.RawMessage
}

func newDPoPTestSigner(t *testing.T) *dpopTestSigner {
	t.Helper()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	rsaJWK, err := json.Marshal(map[string]string{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.E)).Bytes()),
	})
	require.NoError(t, err)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	edJWK, err := json.Marshal(map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(pub),
	})
	require.NoError(t, err)

	return &dpopTestSigner{rsaKey: rsaKey, rsaJWK: rsaJWK, edKey: priv, edJWK: edJWK}
}

// proof builds a signed proof with the given headers and claims. `alg` defaults
// to RS256; pass "EdDSA" to exercise the OKP branch, which is signed with a real
// Ed25519 key so the only thing that can reject it is the algorithm whitelist.
func (s *dpopTestSigner) proof(t *testing.T, alg, htu, jti string, iat time.Time) string {
	t.Helper()

	jwk := s.rsaJWK
	if alg == "EdDSA" {
		jwk = s.edJWK
	}
	if alg == "" {
		alg = "RS256"
	}

	headerJSON, err := json.Marshal(map[string]any{
		"typ": "dpop+jwt",
		"alg": alg,
		"jwk": jwk,
	})
	require.NoError(t, err)
	claimsJSON, err := json.Marshal(map[string]any{
		"jti": jti,
		// DPoP proofs in this suite always target a token-exchange POST.
		"htm": "POST",
		"htu": htu,
		"iat": iat.Unix(),
	})
	require.NoError(t, err)

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)

	var signature []byte
	if alg == "EdDSA" {
		signature = ed25519.Sign(s.edKey, []byte(signingInput))
	} else {
		hashed := sha256.Sum256([]byte(signingInput))
		signature, err = rsa.SignPKCS1v15(rand.Reader, s.rsaKey, crypto.SHA256, hashed[:])
		require.NoError(t, err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func dpopProtocolCode(t *testing.T, err error) string {
	t.Helper()
	var protocolErr apperrors.ProtocolCodeError
	require.True(t, errors.As(err, &protocolErr), "error %v carries no protocol code", err)
	return protocolErr.ProtocolCode()
}

const dpopTestTokenURL = "https://auth.example.com" + TokenEndpointPath

func TestDPoP_ProofWithMatchingHTUValidates(t *testing.T) {
	signer := newDPoPTestSigner(t)
	service := Service{}

	result, err := service.ValidateDPoPProof(context.Background(), ValidateDPoPProofCommand{
		DPoPHeader: signer.proof(t, "", dpopTestTokenURL, "jti-1", time.Now().UTC()),
		Method:     "POST",
		URL:        dpopTestTokenURL,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.JKT, "the JWK thumbprint is what binds the token")
	assert.Equal(t, "jti-1", result.JTI)
}

// The headline defect: the exchange path passed "" as the proof URL, and
// urlMatches is an equality test, so every proof failed and no dpop_required
// application could ever complete a code exchange.
func TestDPoP_ProofHTUMustMatchTheRequestURL(t *testing.T) {
	signer := newDPoPTestSigner(t)
	service := Service{}

	_, err := service.ValidateDPoPProof(context.Background(), ValidateDPoPProofCommand{
		DPoPHeader: signer.proof(t, "", "https://evil.example.com"+TokenEndpointPath, "jti-2", time.Now().UTC()),
		Method:     "POST",
		URL:        dpopTestTokenURL,
	})
	require.Error(t, err)
	assert.Equal(t, "invalid_dpop_proof", dpopProtocolCode(t, err))
}

// An empty proof URL can never match, so a caller must never be able to reach
// validation with one. The helper is what the exchange path uses; it must fall
// back to the issuer-derived token endpoint rather than returning "".
func TestDPoP_TokenEndpointURLIsNeverEmpty(t *testing.T) {
	t.Run("provided URL wins", func(t *testing.T) {
		service := Service{}
		assert.Equal(t, dpopTestTokenURL, service.dpopTokenEndpointURL(ExchangeAuthorizationCodeCommand{
			DPoPURL: dpopTestTokenURL,
		}))
	})

	t.Run("falls back to the issuer", func(t *testing.T) {
		service := Service{}
		got := service.dpopTokenEndpointURL(ExchangeAuthorizationCodeCommand{
			IssuerURL: "https://issuer.example.com/",
		})
		assert.Equal(t, "https://issuer.example.com"+TokenEndpointPath, got)
		// The same string the discovery document advertises.
		discovery, err := service.GetOIDCDiscovery(context.Background(), "https://issuer.example.com/")
		require.NoError(t, err)
		assert.Equal(t, discovery.TokenEndpoint, got)
	})
}

// The whitelist used to live only in DPoPValidator.Validate, which no production
// path called: the exchange path used the package-level function, and its OKP
// branch verified an Ed25519 signature without consulting `alg` at all, so
// `alg: "EdDSA"` was accepted.
func TestDPoP_UnsupportedAlgorithmIsRejectedOnTheProductionPath(t *testing.T) {
	signer := newDPoPTestSigner(t)
	service := Service{}

	_, err := service.ValidateDPoPProof(context.Background(), ValidateDPoPProofCommand{
		DPoPHeader: signer.proof(t, "EdDSA", dpopTestTokenURL, "jti-3", time.Now().UTC()),
		Method:     "POST",
		URL:        dpopTestTokenURL,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
	assert.Equal(t, "invalid_dpop_proof", dpopProtocolCode(t, err))
}

// 14.27.2 pins the window at ±60s. The app layer used to pass 5 minutes, which
// made a proof usable five times longer than the document allows.
func TestDPoP_ClockSkewIsSixtySeconds(t *testing.T) {
	signer := newDPoPTestSigner(t)
	service := Service{}
	now := time.Now().UTC()

	t.Run("within the window", func(t *testing.T) {
		_, err := service.ValidateDPoPProof(context.Background(), ValidateDPoPProofCommand{
			DPoPHeader: signer.proof(t, "", dpopTestTokenURL, "jti-4", now.Add(-30*time.Second)),
			Method:     "POST",
			URL:        dpopTestTokenURL,
		})
		assert.NoError(t, err)
	})

	t.Run("older than the window", func(t *testing.T) {
		_, err := service.ValidateDPoPProof(context.Background(), ValidateDPoPProofCommand{
			DPoPHeader: signer.proof(t, "", dpopTestTokenURL, "jti-5", now.Add(-90*time.Second)),
			Method:     "POST",
			URL:        dpopTestTokenURL,
		})
		require.Error(t, err)
		assert.Equal(t, "invalid_dpop_proof", dpopProtocolCode(t, err))
	})
}

// The replay cache is state, so it only runs through the validator. Before this
// the exchange path bypassed it entirely: the same proof could be replayed.
func TestDPoP_ReplayedJTIIsRejected(t *testing.T) {
	signer := newDPoPTestSigner(t)
	service := Service{DPoP: platformdpop.NewDPoPValidator()}
	proof := signer.proof(t, "", dpopTestTokenURL, "jti-replay", time.Now().UTC())

	command := ValidateDPoPProofCommand{DPoPHeader: proof, Method: "POST", URL: dpopTestTokenURL}

	_, err := service.ValidateDPoPProof(context.Background(), command)
	require.NoError(t, err)

	_, err = service.ValidateDPoPProof(context.Background(), command)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "replay")
}

type dpopReplayStoreFunc func(context.Context, string) (bool, error)

func (f dpopReplayStoreFunc) Check(ctx context.Context, jti string) (bool, error) {
	return f(ctx, jti)
}

func TestDPoP_ReplayStoreFailureRemainsUnavailable(t *testing.T) {
	signer := newDPoPTestSigner(t)
	service := Service{
		DPoP: platformdpop.NewDPoPValidatorWithReplayStore(dpopReplayStoreFunc(
			func(context.Context, string) (bool, error) {
				return false, apperrors.NewUnavailable("replay storage unavailable")
			},
		)),
	}
	proof := signer.proof(t, "", dpopTestTokenURL, "jti-store-failure", time.Now().UTC())

	_, err := service.ValidateDPoPProof(context.Background(), ValidateDPoPProofCommand{
		DPoPHeader: proof,
		Method:     "POST",
		URL:        dpopTestTokenURL,
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeUnavailable, appErr.Code)
	assert.Equal(t, 503, appErr.HTTPStatus())
}

// A nil Service.DPoP must not silently disable replay detection: bootstrap could
// forget the assignment and go build / go vet would not notice.
func TestDPoP_NilValidatorFallsBackToTheDefault(t *testing.T) {
	signer := newDPoPTestSigner(t)
	service := Service{}
	proof := signer.proof(t, "", dpopTestTokenURL, "jti-default", time.Now().UTC())

	command := ValidateDPoPProofCommand{DPoPHeader: proof, Method: "POST", URL: dpopTestTokenURL}

	_, err := service.ValidateDPoPProof(context.Background(), command)
	require.NoError(t, err)

	_, err = service.ValidateDPoPProof(context.Background(), command)
	require.Error(t, err, "the default validator must still catch the replay")
	assert.Contains(t, err.Error(), "replay")
}

func TestDPoP_BindingRoundTripsThroughTokenMetadata(t *testing.T) {
	service := Service{}

	// The exact composition the exchange path performs: the ordinary
	// authorization-code metadata, then the confirmation key on top.
	metadata := service.BindTokenToDPoP(
		newAuthorizationCodeTokenMetadata(&authorizationcodedomain.AuthorizationCode{ID: "code-1"}),
		"thumbprint-value",
	)

	assert.Equal(t, "DPoP", metadata["tokenBinding"])
	assert.Equal(t, "thumbprint-value", dpopThumbprintFromMetadata(metadata))
	// The grant metadata must survive the merge.
	assert.Equal(t, "code-1", metadata["authorizationCodeId"])
}

func TestDpopThumbprintFromMetadata(t *testing.T) {
	cases := map[string]map[string]any{
		"nil metadata":         nil,
		"no cnf":               {"grantType": "authorization_code"},
		"cnf is not an object": {"cnf": "jkt-value"},
		"cnf without jkt":      {"cnf": map[string]any{"other": "x"}},
		"jkt is not a string":  {"cnf": map[string]any{"jkt": 42}},
		"jkt is blank":         {"cnf": map[string]any{"jkt": "   "}},
	}
	for name, metadata := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, dpopThumbprintFromMetadata(metadata))
		})
	}

	t.Run("cnf.jkt", func(t *testing.T) {
		assert.Equal(t, "thumbprint", dpopThumbprintFromMetadata(map[string]any{
			"cnf": map[string]any{"jkt": "thumbprint"},
		}))
	})
}

// The advertised set must be the enforced set. 14.27.1 records the drift between
// the whitelist, the document and the code as one of's parts, so this is
// the guard against it coming back.
func TestDPoP_DiscoveryAdvertisesTheEnforcedAlgorithms(t *testing.T) {
	service := Service{}

	discovery, err := service.GetOIDCDiscovery(context.Background(), "https://auth.example.com")
	require.NoError(t, err)

	assert.Equal(t, platformdpop.SupportedAlgorithms, discovery.DPoPSigningAlgValuesSupported)
	assert.NotEmpty(t, discovery.DPoPSigningAlgValuesSupported)

	// And the validator's whitelist is the same set, element for element.
	validator := platformdpop.NewDPoPValidator()
	require.Len(t, validator.AllowedAlgorithms, len(platformdpop.SupportedAlgorithms))
	for _, alg := range platformdpop.SupportedAlgorithms {
		assert.Contains(t, validator.AllowedAlgorithms, alg)
	}
}

// The discovery slice must not be the package-level slice: a caller could then
// mutate the validator's source of truth through a response struct.
func TestDPoP_DiscoverySliceIsACopy(t *testing.T) {
	service := Service{}

	discovery, err := service.GetOIDCDiscovery(context.Background(), "https://auth.example.com")
	require.NoError(t, err)
	require.NotEmpty(t, discovery.DPoPSigningAlgValuesSupported)

	original := platformdpop.SupportedAlgorithms[0]
	discovery.DPoPSigningAlgValuesSupported[0] = "mutated"
	t.Cleanup(func() { platformdpop.SupportedAlgorithms[0] = original })

	assert.Equal(t, original, platformdpop.SupportedAlgorithms[0])
}
