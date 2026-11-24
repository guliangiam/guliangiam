package dpop

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// SupportedAlgorithms is the single source of truth for the DPoP proof signing
// algorithms this server accepts. It drives both the validator's whitelist and
// the discovery document's `dpop_signing_alg_values_supported`, so the advertised
// set cannot drift from the enforced one — one of the four convergence items of
// 设计报告 14.27.1.
//
// These three are the implementation's actual capability. Tightening the set to
// PS256/ES256 as 设计报告 14.27.2 sketches is a breaking change for any client already
// sending RS256 proofs, so it has to go through the 手册 4.2 deprecation flow
// rather than land silently here.
var SupportedAlgorithms = []string{"RS256", "ES256", "PS256"}

const (
	// ProofMaxAge is the RFC 9449 §11.1 acceptance window for a proof's `iat`.
	ProofMaxAge = 60 * time.Second
	// ProofClockSkew tolerates a proof whose `iat` is slightly in the future, and
	// is deliberately equal to ProofMaxAge so the window is symmetric (±60s).
	ProofClockSkew = 60 * time.Second
	// replayCacheMaxEntries bounds the in-process replay cache. Unbounded, the
	// map grows with every distinct `jti`, so a client that mints a fresh one per
	// request is a memory exhaustion vector.
	replayCacheMaxEntries = 1 << 16
)

// SupportedAlgorithmsSet returns SupportedAlgorithms as a lookup set.
func SupportedAlgorithmsSet() map[string]struct{} {
	set := make(map[string]struct{}, len(SupportedAlgorithms))
	for _, alg := range SupportedAlgorithms {
		set[alg] = struct{}{}
	}
	return set
}

func isSupportedAlgorithm(alg string) bool {
	for _, candidate := range SupportedAlgorithms {
		if candidate == alg {
			return true
		}
	}
	return false
}

type DPoPValidator struct {
	AllowedAlgorithms map[string]struct{}
	MaxProofAge       time.Duration
	ReplayStore       ReplayStore
	Nonces            NonceStore // optional: nil means nonce challenge is not enforced
}

func NewDPoPValidator() *DPoPValidator {
	return &DPoPValidator{
		AllowedAlgorithms: SupportedAlgorithmsSet(),
		MaxProofAge:       ProofMaxAge,
		ReplayStore:       NewReplayCache(),
		Nonces:            NewLocalNonceStore(),
	}
}

// NewDPoPValidatorWithNonceStore overrides the nonce store — useful when
// bootstrap wants to share one store across multiple validators (e.g. tests).
func NewDPoPValidatorWithNonceStore(nonces NonceStore) *DPoPValidator {
	v := NewDPoPValidator()
	v.Nonces = nonces
	return v
}

// NewDPoPValidatorWithReplayStore overrides the replay store — useful when
// bootstrap wants to use a shared Postgres/Redis store across processes.
func NewDPoPValidatorWithReplayStore(store ReplayStore) *DPoPValidator {
	v := NewDPoPValidator()
	v.ReplayStore = store
	return v
}

// Validate applies the checks that need state: the configured algorithm
// whitelist and replay detection. The stateless checks — including the
// package-wide algorithm whitelist — live in ValidateDPoPProof, so a caller that
// uses the package-level function (as the token-exchange path did) is not
// silently left without them.
func (v *DPoPValidator) Validate(ctx context.Context, proofJWT, method, url string) (*Proof, error) {
	proof, err := ValidateDPoPProof(proofJWT, method, url, v.MaxProofAge)
	if err != nil {
		return nil, err
	}

	if _, ok := v.AllowedAlgorithms[proof.Header.Alg]; !ok {
		return nil, apperrors.NewInvalidArgumentf("DPoP proof algorithm %q is not allowed", proof.Header.Alg)
	}

	if proof.Claims.Nonce != "" && v.Nonces != nil {
		if err := v.Nonces.Verify(ctx, proof.Claims.Nonce); err != nil {
			return nil, err // surfaces "not recognized" / "expired" per NonceStore
		}
	}

	if v.ReplayStore != nil {
		firstSeen, err := v.ReplayStore.Check(ctx, proof.Claims.JTI)
		if err != nil {
			return nil, apperrors.WrapUnavailable(err, "DPoP replay store unavailable")
		}
		if !firstSeen {
			return nil, apperrors.NewInvalidArgument("DPoP proof replay detected")
		}
	}

	return proof, nil
}

// ReplayStore records and queries `jti` presence. A shared implementation
// (PostgreSQL/Redis) catches replays across replicas; an in-process one
// (ReplayCache below) is fine for single-instance or test environments.
type ReplayStore interface {
	// Check records jti and reports whether this is the first time it was seen.
	// Returns false, nil when the jti already exists (replay), or an error when
	// replay detection could not be completed.
	Check(ctx context.Context, jti string) (bool, error)
}

// ReplayCache is a process-local `jti` set. It is deliberately not the final
// answer: RFC 9449 §11.1 wants a shared store so a replay is caught on any
// replica, and 14.27.2 specifies Redis for it. Until that lands, a multi-replica
// deployment shares no replay state.
type ReplayCache struct {
	mu         sync.RWMutex
	seen       map[string]time.Time
	maxAge     time.Duration
	maxEntries int
}

func NewReplayCache() *ReplayCache {
	// 14.27.2: TTL = 证明最大窗口 60s + 偏移 120s.
	rc := &ReplayCache{
		seen:       make(map[string]time.Time),
		maxAge:     ProofMaxAge + 2*ProofClockSkew,
		maxEntries: replayCacheMaxEntries,
	}
	recover.SafeGo("dpop-replay-evict", rc.evictLoop)
	return rc
}

// Check records jti and reports whether this is the first time it was seen.
func (rc *ReplayCache) Check(_ context.Context, jti string) (bool, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.seen == nil {
		rc.seen = make(map[string]time.Time)
	}
	if rc.maxEntries <= 0 {
		rc.maxEntries = replayCacheMaxEntries
	}

	now := time.Now()
	if _, exists := rc.seen[jti]; exists {
		return false, nil
	}
	if len(rc.seen) >= rc.maxEntries {
		rc.evictLocked(now)
		if len(rc.seen) >= rc.maxEntries {
			return false, apperrors.NewUnavailable("DPoP replay cache is full")
		}
	}
	rc.seen[jti] = now
	return true, nil
}

func (rc *ReplayCache) evictLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rc.mu.Lock()
		rc.evictLocked(time.Now())
		rc.mu.Unlock()
	}
}

// evictLocked drops entries whose replay window has expired. If the cache is
// still full, Check rejects new proofs rather than evicting live replay records.
func (rc *ReplayCache) evictLocked(now time.Time) {
	for jti, seenAt := range rc.seen {
		if now.Sub(seenAt) > rc.maxAge {
			delete(rc.seen, jti)
		}
	}
}

// Proof represents a parsed DPoP proof JWT as defined in RFC 9449.
type Proof struct {
	Header  ProofHeader
	Claims  ProofClaims
	Raw     string
	JWKJSON string
}

// ProofHeader contains the JWT header fields of a DPoP proof.
type ProofHeader struct {
	Typ string          `json:"typ"`
	Alg string          `json:"alg"`
	JWK json.RawMessage `json:"jwk"`
	Kid string          `json:"kid,omitempty"`
}

// ProofClaims contains the JWT payload claims of a DPoP proof.
type ProofClaims struct {
	JTI string `json:"jti"`
	HTM string `json:"htm"`
	HTU string `json:"htu"`
	IAT int64  `json:"iat"`
	// Optional claims
	Ath   string `json:"ath,omitempty"`
	Cnf   string `json:"cnf,omitempty"`
	Nonce string `json:"nonce,omitempty"`
}

// ValidateDPoPProof validates a DPoP proof JWT from the HTTP header.
// It verifies the signature, checks required claims, and validates timing.
func ValidateDPoPProof(proofJWT, method, url string, maxClockSkew time.Duration) (*Proof, error) {
	if strings.TrimSpace(proofJWT) == "" {
		return nil, apperrors.NewInvalidArgument("DPoP proof is required")
	}

	parts := strings.Split(proofJWT, ".")
	if len(parts) != 3 {
		return nil, apperrors.NewInvalidArgument("invalid DPoP proof format")
	}

	// Parse header.
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode DPoP proof header")
	}
	var header ProofHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse DPoP proof header")
	}

	// Validate header fields.
	if header.Typ != "dpop+jwt" {
		return nil, apperrors.NewInvalidArgument("DPoP proof header typ must be dpop+jwt")
	}
	if strings.TrimSpace(header.Alg) == "" {
		return nil, apperrors.NewInvalidArgument("DPoP proof header alg is required")
	}
	if len(header.JWK) == 0 {
		return nil, apperrors.NewInvalidArgument("DPoP proof header jwk is required")
	}
	// The whitelist belongs here, in the stateless function, not only in
	// DPoPValidator.Validate: verifyProofSignature used to accept any `alg`
	// string on the ECDSA and Ed25519 branches, so a proof declaring `EdDSA` or
	// `ES384` verified fine as long as the key parsed — and the token-exchange
	// path called this function directly, never the validator.
	if !isSupportedAlgorithm(header.Alg) {
		return nil, apperrors.NewInvalidArgumentf("DPoP proof algorithm %q is not supported", header.Alg)
	}

	// Parse claims.
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode DPoP proof claims")
	}
	var claims ProofClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse DPoP proof claims")
	}

	// Validate required claims.
	if strings.TrimSpace(claims.JTI) == "" {
		return nil, apperrors.NewInvalidArgument("DPoP proof jti claim is required")
	}
	if strings.TrimSpace(claims.HTM) == "" {
		return nil, apperrors.NewInvalidArgument("DPoP proof htm claim is required")
	}
	if strings.TrimSpace(claims.HTU) == "" {
		return nil, apperrors.NewInvalidArgument("DPoP proof htu claim is required")
	}
	if claims.IAT == 0 {
		return nil, apperrors.NewInvalidArgument("DPoP proof iat claim is required")
	}

	// Verify HTTP method matches. A mismatch is the client's mistake, not ours:
	// returning `internal` here meant a bad proof answered 500.
	if !strings.EqualFold(claims.HTM, method) {
		return nil, apperrors.NewInvalidArgument("DPoP proof htm does not match request method")
	}

	// Verify URL matches (compare without trailing slashes and scheme-insensitive).
	if !urlMatches(claims.HTU, url) {
		return nil, apperrors.NewInvalidArgument("DPoP proof htu does not match request URL")
	}

	// Verify timing: iat should be within acceptable clock skew.
	issuedAt := time.Unix(claims.IAT, 0)
	now := time.Now().UTC()
	if issuedAt.Add(maxClockSkew).Before(now) {
		return nil, apperrors.NewInvalidArgument("DPoP proof iat is too old")
	}
	if issuedAt.Add(-maxClockSkew).After(now) {
		return nil, apperrors.NewInvalidArgument("DPoP proof iat is in the future")
	}

	proof := &Proof{
		Header:  header,
		Claims:  claims,
		Raw:     proofJWT,
		JWKJSON: string(header.JWK),
	}

	// Verify the signature using the JWK from the header.
	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode DPoP proof signature")
	}

	publicKey, err := parseJWKPublicKey(header.JWK)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse DPoP proof JWK")
	}

	if err := verifyProofSignature(signingInput, sigBytes, publicKey, header.Alg); err != nil {
		return nil, apperrors.WrapInternalf(err, "DPoP proof signature verification failed")
	}

	return proof, nil
}

// jwkMemberKTY 是 RFC 7638 规范成员中的 key type 字段名。
const jwkMemberKTY = "kty"

// ComputeJWKThumbprint computes the SHA-256 JWK thumbprint per RFC 7638.
// The thumbprint is a base64url-encoded SHA-256 hash of the canonical JWK form.
func ComputeJWKThumbprint(jwkJSON string) (string, error) {
	// Parse the JWK to determine key type.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jwkJSON), &raw); err != nil {
		return "", apperrors.WrapInternalf(err, "parse JWK")
	}

	var kty string
	if ktyRaw, ok := raw[jwkMemberKTY]; ok {
		if err := json.Unmarshal(ktyRaw, &kty); err != nil {
			return "", apperrors.WrapInternalf(err, "parse kty")
		}
	}

	// Build canonical form per RFC 7638 Section 3.2.
	// Required members depend on key type:
	// - EC: crv, kty, x, y
	// - RSA: e, kty, n
	// - OKP: crv, kty, x
	var canonicalMembers []string
	switch kty {
	case "EC":
		canonicalMembers = []string{"crv", jwkMemberKTY, "x", "y"}
	case "RSA":
		canonicalMembers = []string{"e", jwkMemberKTY, "n"}
	case "OKP":
		canonicalMembers = []string{"crv", jwkMemberKTY, "x"}
	default:
		return "", apperrors.NewInvalidArgumentf("unsupported JWK key type: %s", kty)
	}

	// Build the canonical JSON object with members in lexicographic order.
	canonical := make(map[string]string, len(canonicalMembers))
	for _, member := range canonicalMembers {
		if rawVal, ok := raw[member]; ok {
			var val string
			if err := json.Unmarshal(rawVal, &val); err != nil {
				return "", apperrors.WrapInternalf(err, "parse JWK member %s", member)
			}
			canonical[member] = val
		}
	}

	// Serialize in lexicographic key order.
	canonicalJSON, err := json.Marshal(canonical)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "marshal canonical JWK")
	}

	// Compute SHA-256 hash.
	sum := sha256.Sum256(canonicalJSON)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// BindTokenToDPoP marks a token as DPoP-bound by recording the JWK thumbprint.
// The thumbprint is stored in the token metadata under the "cnf" (confirmation) claim.
func BindTokenToDPoP(metadata map[string]any, jwkThumbprint string) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadata["cnf"] = map[string]any{
		"jkt": jwkThumbprint,
	}
	metadata["tokenBinding"] = "DPoP"
	return metadata
}

// urlMatches compares two URLs for DPoP htu validation.
// It normalizes trailing slashes and does case-insensitive scheme/host comparison.
func urlMatches(htu, requestURL string) bool {
	htu = strings.TrimRight(strings.TrimSpace(htu), "/")
	requestURL = strings.TrimRight(strings.TrimSpace(requestURL), "/")
	return strings.EqualFold(htu, requestURL)
}

// parseJWKPublicKey parses a JWK and returns the corresponding public key.
func parseJWKPublicKey(jwkJSON json.RawMessage) (any, error) {
	var jwk struct {
		Kty string `json:"kty"`
		Crv string `json:"crv,omitempty"`
		N   string `json:"n,omitempty"`
		E   string `json:"e,omitempty"`
		X   string `json:"x,omitempty"`
		Y   string `json:"y,omitempty"`
	}
	if err := json.Unmarshal(jwkJSON, &jwk); err != nil {
		return nil, apperrors.WrapInternalf(err, "unmarshal JWK")
	}

	switch jwk.Kty {
	case "RSA":
		nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "decode RSA n")
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "decode RSA e")
		}
		n := new(big.Int).SetBytes(nBytes)
		e := new(big.Int).SetBytes(eBytes)
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil

	case "EC":
		xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "decode EC x")
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "decode EC y")
		}
		var curve elliptic.Curve
		switch jwk.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, apperrors.NewInvalidArgumentf("unsupported EC curve: %s", jwk.Crv)
		}
		// Rebuild the SEC1 uncompressed point (0x04 || X || Y) and let the
		// standard parser validate that it lies on the curve. Constructing
		// ecdsa.PublicKey{X, Y} from raw big.Ints is deprecated because it
		// bypasses those checks.
		size := (curve.Params().BitSize + 7) / 8
		if len(xBytes) > size || len(yBytes) > size {
			return nil, apperrors.NewInvalidArgumentf("invalid EC public key coordinates")
		}
		point := make([]byte, 1+2*size)
		point[0] = 0x04
		copy(point[1+size-len(xBytes):1+size], xBytes)
		copy(point[1+2*size-len(yBytes):], yBytes)
		pub, err := ecdsa.ParseUncompressedPublicKey(curve, point)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "parse EC public key")
		}
		return pub, nil

	case "OKP":
		xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "decode OKP x")
		}
		return ed25519.PublicKey(xBytes), nil

	default:
		return nil, apperrors.NewInvalidArgumentf("unsupported JWK key type: %s", jwk.Kty)
	}
}

// verifyProofSignature verifies the signature of a DPoP proof JWT. The caller
// has already checked `alg` against SupportedAlgorithms; the per-branch checks
// here tie the algorithm to the key material so a proof cannot pick a stronger
// key type than the algorithm it advertises.
func verifyProofSignature(signingInput string, signature []byte, publicKey any, alg string) error {
	switch key := publicKey.(type) {
	case *rsa.PublicKey:
		hashed := sha256.Sum256([]byte(signingInput))
		switch alg {
		case "RS256":
			return rsa.VerifyPKCS1v15(key, crypto.SHA256, hashed[:], signature)
		case "PS256":
			return rsa.VerifyPSS(key, crypto.SHA256, hashed[:], signature, nil)
		default:
			return apperrors.NewInvalidArgumentf("algorithm %s is not valid for an RSA JWK", alg)
		}
	case *ecdsa.PublicKey:
		if alg != "ES256" {
			return apperrors.NewInvalidArgumentf("algorithm %s is not valid for an EC JWK", alg)
		}
		if key.Curve != elliptic.P256() {
			// ES256 is defined over P-256 only (RFC 7518 §3.4).
			return apperrors.NewInvalidArgument("DPoP ES256 proof must use the P-256 curve")
		}
		// RFC 7518 §3.4: a JWS ES256 signature is the raw R||S concatenation,
		// each half exactly the coordinate size (32 bytes for P-256). It is not
		// DER and not JSON — the previous `json.Unmarshal` of {r,s} could never
		// succeed against a real proof, so ES256 was advertised but unusable.
		if len(signature) != 64 {
			return apperrors.NewInvalidArgumentf("DPoP ES256 signature must be 64 bytes, got %d", len(signature))
		}
		hashed := sha256.Sum256([]byte(signingInput))
		r := new(big.Int).SetBytes(signature[:32])
		s := new(big.Int).SetBytes(signature[32:])
		if !ecdsa.Verify(key, hashed[:], r, s) {
			return apperrors.NewUnauthorized("ECDSA signature verification failed")
		}
		return nil
	default:
		// An Ed25519 key would previously be accepted here without checking
		// `alg` at all (EdDSA, ES384, ES512 all took this branch). The supported
		// set does not contain any OKP algorithm, so refuse the key type rather
		// than verify a proof under an algorithm we never announced.
		return apperrors.NewInvalidArgumentf("unsupported DPoP proof key type: %T", publicKey)
	}
}
