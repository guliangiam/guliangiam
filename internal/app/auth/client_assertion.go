package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// maxClientAssertionBytes bounds the JWT before any decoding so a
	// multi-megabyte assertion cannot burn CPU in base64/JSON parsing.
	maxClientAssertionBytes = 16 * 1024
	// clientAssertionMaxLifetime is the RFC 7523 §3 upper bound for an
	// assertion's useful window (exp - iat / exp - now).
	clientAssertionMaxLifetime = 5 * time.Minute
	// clientAssertionClockSkew tolerates clock drift between client and server.
	clientAssertionClockSkew = 60 * time.Second
	// clientAssertionReplayPrefix namespaces jti values so a JWT jti can never
	// collide with a DPoP or SAML identifier in a shared store.
	clientAssertionReplayPrefix = "client_assertion:"
)

// AssertionReplayStore records a client assertion jti for exactly the
// assertion's remaining lifetime. The signature matches the SAML replay store
// so bootstrap can wire the same Redis/process-local implementation.
type AssertionReplayStore interface {
	Reserve(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// parsedClientAssertion is the verified-shape representation used by the
// signature check. Nothing here is trusted until verifyClientAssertionSignature
// succeeds.
type parsedClientAssertion struct {
	Alg          string
	Kid          string
	Iss          string
	Sub          string
	Audience     []string
	Exp          time.Time
	IssuedAt     *time.Time
	Jti          string
	SigningInput string
	Signature    []byte
}

// ClientJWKSFetcher retrieves keys from a client's remote jwks_uri.
type ClientJWKSFetcher interface {
	Fetch(ctx context.Context, jwksURI string) ([]*applicationdomain.ClientJWK, error)
}

// verifyPrivateKeyJWTAssertion validates a private_key_jwt client assertion
// (OIDC Core 1.0 §9 / RFC 7523 §2.2) against the client's registered JWKS.
//
// Every failure returns the same 401 message: an unknown kid, a wrong
// signature, a bad audience and a replayed jti must be indistinguishable to
// the caller. The lookup uses the authenticated application's organization,
// never a tenant id carried in the assertion.
func (s *Service) verifyPrivateKeyJWTAssertion(
	ctx context.Context,
	tx appuow.UnitOfWork,
	application *applicationdomain.Application,
	assertion string,
	assertionType string,
	audience string,
) error {
	failure := apperrors.NewUnauthorized(clientAuthFailureMessage)

	if application == nil {
		return failure
	}
	if strings.TrimSpace(assertionType) != ClientAssertionTypeJWT {
		return failure
	}
	if len(assertion) == 0 || len(assertion) > maxClientAssertionBytes {
		return failure
	}
	if !isAbsoluteHTTPURL(audience) {
		return failure
	}

	parsed, err := parseClientAssertion(assertion)
	if err != nil {
		return failure
	}
	if !isSupportedClientAssertionAlgorithm(parsed.Alg) {
		return failure
	}
	if parsed.Iss != strings.TrimSpace(application.ClientID) || parsed.Sub != strings.TrimSpace(application.ClientID) {
		return failure
	}
	if parsed.Jti == "" {
		return failure
	}

	now := s.Clock.Now().UTC()
	if !parsed.Exp.After(now) {
		return failure
	}
	if parsed.Exp.Sub(now) > clientAssertionMaxLifetime+clientAssertionClockSkew {
		return failure
	}
	if parsed.IssuedAt != nil {
		if parsed.IssuedAt.After(now.Add(clientAssertionClockSkew)) {
			return failure
		}
		if parsed.Exp.Sub(*parsed.IssuedAt) > clientAssertionMaxLifetime+clientAssertionClockSkew {
			return failure
		}
	}
	if !audienceContains(parsed.Audience, audience) {
		return failure
	}

	keys, err := tx.Applications().ListActiveClientJWKs(ctx, application.OrganizationID, application.ID)
	if err != nil {
		return failure
	}
	var key *applicationdomain.ClientJWK
	for _, candidate := range keys {
		if candidate != nil && candidate.Kid == parsed.Kid {
			key = candidate
			break
		}
	}
	// No inline key matched: fall back to the client's remote jwks_uri, if any.
	if key == nil && s.ClientJWKSFetcher != nil {
		if uriRepo, ok := tx.Applications().(applicationdomain.ClientJWKSURIRepository); ok {
			uri, uriErr := uriRepo.GetClientJWKSURI(ctx, application.OrganizationID, application.ID)
			if uriErr == nil && uri != nil && uri.JWKSURI != "" {
				fetched, fetchErr := s.ClientJWKSFetcher.Fetch(ctx, uri.JWKSURI)
				if fetchErr != nil {
					return failure
				}
				for _, candidate := range fetched {
					if candidate != nil && candidate.Kid == parsed.Kid {
						key = candidate
						break
					}
				}
			}
		}
	}
	// The kid must belong to this client: falling back to any other key (or the
	// platform signing key) would let a client assert with material it does not
	// own.
	if key == nil || key.Alg != parsed.Alg {
		return failure
	}
	if err := verifyClientAssertionSignature(parsed, key.JWK); err != nil {
		return failure
	}

	ttl := parsed.Exp.Sub(now) + clientAssertionClockSkew
	if ttl <= 0 {
		ttl = clientAssertionMaxLifetime + clientAssertionClockSkew
	}
	firstSeen, err := s.clientAssertionReplayStore().Reserve(
		ctx,
		clientAssertionReplayPrefix+strings.TrimSpace(application.ClientID)+":"+parsed.Jti,
		ttl,
	)
	if err != nil || !firstSeen {
		return failure
	}
	return nil
}

func parseClientAssertion(raw string) (*parsedClientAssertion, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, apperrors.NewInvalidArgument("client assertion must have three segments")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, apperrors.NewInvalidArgument("client assertion header is not base64url")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, apperrors.NewInvalidArgument("client assertion header is not JSON")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, apperrors.NewInvalidArgument("client assertion claims are not base64url")
	}
	var claims map[string]json.RawMessage
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, apperrors.NewInvalidArgument("client assertion claims are not JSON")
	}

	assertion := &parsedClientAssertion{
		Alg:          strings.TrimSpace(header.Alg),
		Kid:          strings.TrimSpace(header.Kid),
		SigningInput: parts[0] + "." + parts[1],
	}
	if assertion.Alg == "" || assertion.Kid == "" {
		return nil, apperrors.NewInvalidArgument("client assertion header must declare alg and kid")
	}

	if assertion.Iss, err = claimString(claims, "iss"); err != nil {
		return nil, err
	}
	if assertion.Sub, err = claimString(claims, "sub"); err != nil {
		return nil, err
	}
	if assertion.Jti, err = claimString(claims, "jti"); err != nil {
		return nil, err
	}
	if assertion.Audience, err = claimAudience(claims["aud"]); err != nil {
		return nil, err
	}
	expiration, err := claimTime(claims, "exp", true)
	if err != nil {
		return nil, err
	}
	if expiration == nil {
		return nil, apperrors.NewInvalidArgument("client assertion is missing exp")
	}
	assertion.Exp = *expiration
	if issuedAt, timeErr := claimTime(claims, "iat", false); timeErr != nil {
		return nil, timeErr
	} else if issuedAt != nil {
		assertion.IssuedAt = issuedAt
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) == 0 {
		return nil, apperrors.NewInvalidArgument("client assertion signature is not base64url")
	}
	assertion.Signature = signature
	return assertion, nil
}

func claimString(claims map[string]json.RawMessage, name string) (string, error) {
	raw, ok := claims[name]
	if !ok {
		return "", apperrors.NewInvalidArgument("client assertion is missing " + name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", apperrors.NewInvalidArgument("client assertion " + name + " must be a string")
	}
	return strings.TrimSpace(value), nil
}

func claimAudience(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, apperrors.NewInvalidArgument("client assertion is missing aud")
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if strings.TrimSpace(single) == "" {
			return nil, apperrors.NewInvalidArgument("client assertion aud must not be empty")
		}
		return []string{strings.TrimSpace(single)}, nil
	}
	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err != nil {
		return nil, apperrors.NewInvalidArgument("client assertion aud must be a string or string array")
	}
	for _, value := range multiple {
		if strings.TrimSpace(value) == "" {
			return nil, apperrors.NewInvalidArgument("client assertion aud must not contain empty values")
		}
	}
	return multiple, nil
}

func claimTime(claims map[string]json.RawMessage, name string, required bool) (*time.Time, error) {
	raw, ok := claims[name]
	if !ok {
		if required {
			return nil, apperrors.NewInvalidArgument("client assertion is missing " + name)
		}
		return nil, nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return nil, apperrors.NewInvalidArgument("client assertion " + name + " must be an integer epoch seconds")
	}
	if _, err := number.Int64(); err != nil {
		return nil, apperrors.NewInvalidArgument("client assertion " + name + " must be an integer epoch seconds")
	}
	value, err := number.Int64()
	if err != nil {
		return nil, apperrors.NewInvalidArgument("client assertion " + name + " must be an integer epoch seconds")
	}
	parsed := time.Unix(value, 0).UTC()
	return &parsed, nil
}

func isSupportedClientAssertionAlgorithm(alg string) bool {
	switch alg {
	case "RS256", "PS256", "ES256":
		return true
	default:
		// `none` and HS* must never reach signature verification: HS* would
		// treat a public JWK as an HMAC secret, and none has no signature at all.
		return false
	}
}

func audienceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func isAbsoluteHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return false
	}
	return parsed.Host != ""
}

func verifyClientAssertionSignature(assertion *parsedClientAssertion, jwkJSON string) error {
	var material struct {
		Kty string `json:"kty"`
		N   string `json:"n"`
		E   string `json:"e"`
		Crv string `json:"crv"`
		X   string `json:"x"`
		Y   string `json:"y"`
	}
	if err := json.Unmarshal([]byte(jwkJSON), &material); err != nil {
		return apperrors.NewInvalidArgument("stored jwk is not JSON")
	}

	digest := sha256.Sum256([]byte(assertion.SigningInput))

	switch assertion.Alg {
	case "RS256", "PS256":
		nBytes, err := base64.RawURLEncoding.DecodeString(material.N)
		if err != nil || len(nBytes) == 0 {
			return apperrors.NewInvalidArgument("stored RSA modulus is invalid")
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(material.E)
		if err != nil || len(eBytes) == 0 || len(eBytes) > 8 {
			return apperrors.NewInvalidArgument("stored RSA exponent is invalid")
		}
		exponent := int(new(big.Int).SetBytes(eBytes).Int64())
		if exponent < 3 {
			return apperrors.NewInvalidArgument("stored RSA exponent is invalid")
		}
		publicKey := &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: exponent,
		}
		if assertion.Alg == "PS256" {
			options := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256}
			if err := rsa.VerifyPSS(publicKey, crypto.SHA256, digest[:], assertion.Signature, options); err != nil {
				return apperrors.NewInvalidArgument("client assertion signature is invalid")
			}
			return nil
		}
		if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], assertion.Signature); err != nil {
			return apperrors.NewInvalidArgument("client assertion signature is invalid")
		}
		return nil

	case "ES256":
		xBytes, err := base64.RawURLEncoding.DecodeString(material.X)
		if err != nil || len(xBytes) == 0 {
			return apperrors.NewInvalidArgument("stored EC x coordinate is invalid")
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(material.Y)
		if err != nil || len(yBytes) == 0 {
			return apperrors.NewInvalidArgument("stored EC y coordinate is invalid")
		}
		if len(assertion.Signature) != 64 {
			return apperrors.NewInvalidArgument("ES256 signature must be 64 bytes")
		}
		publicKey := &ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}
		r := new(big.Int).SetBytes(assertion.Signature[:32])
		sig := new(big.Int).SetBytes(assertion.Signature[32:])
		if !ecdsa.Verify(publicKey, digest[:], r, sig) {
			return apperrors.NewInvalidArgument("client assertion signature is invalid")
		}
		return nil

	default:
		return apperrors.NewInvalidArgument("client assertion algorithm is not supported")
	}
}

// clientAssertionReplayStore returns the wired store, or a process-local
// fallback for tests and single-instance runs. The fallback implements the same
// TTL contract, so a replay within the assertion lifetime is still rejected.
func (s *Service) clientAssertionReplayStore() AssertionReplayStore {
	if s.ClientAssertionReplayStore != nil {
		return s.ClientAssertionReplayStore
	}
	return defaultClientAssertionReplayStore
}

var defaultClientAssertionReplayStore = newMemoryAssertionReplayStore()

type memoryAssertionReplayStore struct {
	mu         sync.Mutex
	entries    map[string]time.Time
	maxEntries int
}

func newMemoryAssertionReplayStore() *memoryAssertionReplayStore {
	return &memoryAssertionReplayStore{entries: make(map[string]time.Time), maxEntries: 100_000}
}

func (m *memoryAssertionReplayStore) Reserve(_ context.Context, key string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if len(m.entries) >= m.maxEntries {
		for existing, expiry := range m.entries {
			if expiry.Before(now) {
				delete(m.entries, existing)
			}
		}
	}
	if expiry, exists := m.entries[key]; exists && expiry.After(now) {
		return false, nil
	}
	m.entries[key] = now.Add(ttl)
	return true, nil
}
