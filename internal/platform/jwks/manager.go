package jwks

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	SigningAlgorithmRS256  = "RS256"
	SigningAlgorithmSM2SM3 = "SM2-SM3"

	// sm2CoordinateSize is the SM2 (sm2p256v1) field element size in bytes.
	// RFC 7518 §6.2.1.2 requires an EC JWK coordinate octet string to be exactly
	// the curve size; big.Int.Bytes() drops leading zero bytes and would publish
	// a shorter coordinate that strict JOSE consumers reject.
	sm2CoordinateSize = 32
)

// KeyPair holds an RSA key pair with its key ID.
type KeyPair struct {
	Kid       string
	Algorithm string
	Key       *rsa.PrivateKey
}

// GMKeyPair holds an SM2 key pair with its key ID. The private key is opaque
// to callers; signing goes through platform/crypto.CryptoSuite.
type GMKeyPair struct {
	Kid        string
	Algorithm  string
	PrivateKey *platformcrypto.SM2PrivateKey
	PublicKey  *platformcrypto.SM2PublicKey
}

// Manager manages signing keys for JWT token issuance and JWKS endpoint.
// Without a key ring it generates an RSA-2048 key pair on first access and
// caches it in memory. With a key ring (NewManagerWithKeyRing) it never
// generates keys: the deployment-provided set is authoritative.
type Manager struct {
	mu        sync.RWMutex
	keyPair   *KeyPair
	gmKeyPair *GMKeyPair
	// retired keeps keys that are no longer used for signing but must still
	// verify tokens minted before a rotation (ADR-015). RSA and SM2 have
	// independent retired sets.
	retired   []*KeyPair
	gmRetired []*GMKeyPair
	keySize   int
	kidFn     func() (string, error)

	// keyRing is true when the manager was built from a persisted, deployment-
	// provided key set. It disables runtime key generation.
	keyRing bool
	// retireExpiry maps a retired kid to the last instant it remains valid for
	// verification and JWKS publication (retired_at + overlap). The map is
	// immutable after construction, so lookups need no lock.
	retireExpiry map[string]time.Time
	// now is injectable for tests; production managers use time.Now.
	now func() time.Time
}

// NewManager creates a new JWKS key manager.
// keySize is the RSA key size in bits (recommended: 2048 or 4096).
// kidFn generates the key ID; if nil, a random 128-bit hex string is used.
func NewManager(keySize int, kidFn func() (string, error)) *Manager {
	if kidFn == nil {
		kidFn = randomKid
	}
	return &Manager{
		keySize: keySize,
		kidFn:   kidFn,
		now:     time.Now,
	}
}

// NewManagerWithKeyRing creates a manager backed by a persistent key ring.
// Unlike NewManager, an algorithm family missing from the ring is not generated
// on demand: a key ring is authoritative, so signing with an absent family
// fails closed instead of minting with a process-local ephemeral key.
func NewManagerWithKeyRing(ring *KeyRing) (*Manager, error) {
	if ring == nil || len(ring.keys) == 0 {
		return nil, apperrors.NewInvalidArgument("JWT signing key ring is empty")
	}

	manager := &Manager{
		keyRing:      true,
		retireExpiry: make(map[string]time.Time, len(ring.keys)),
		now:          time.Now,
	}
	for _, key := range ring.keys {
		switch key.algorithm {
		case SigningAlgorithmRS256:
			if key.rsa == nil {
				return nil, apperrors.NewInvalidArgumentf(
					"JWT signing key ring key %q has no RSA private key", key.kid)
			}
			pair := &KeyPair{Kid: key.kid, Algorithm: key.algorithm, Key: key.rsa}
			if key.active {
				manager.keyPair = pair
			} else {
				manager.retired = append(manager.retired, pair)
			}
		case SigningAlgorithmSM2SM3:
			if key.gm == nil {
				return nil, apperrors.NewInvalidArgumentf(
					"JWT signing key ring key %q has no SM2 private key", key.kid)
			}
			pair := &GMKeyPair{
				Kid:        key.kid,
				Algorithm:  key.algorithm,
				PrivateKey: key.gm,
				PublicKey:  key.gm.Public(),
			}
			if key.active {
				manager.gmKeyPair = pair
			} else {
				manager.gmRetired = append(manager.gmRetired, pair)
			}
		default:
			return nil, apperrors.NewInvalidArgumentf(
				"JWT signing key ring key %q has unsupported algorithm %q", key.kid, key.algorithm)
		}
		if key.retiredAt != nil {
			manager.retireExpiry[key.kid] = key.retiredAt.Add(ring.overlap)
		}
	}
	return manager, nil
}

// GetKeyPair returns the current RSA signing key pair. In key-ring mode there
// is no generation fallback: a missing active RS256 key is a configuration
// error, not a reason to mint with a process-local key.
func (m *Manager) GetKeyPair() (*KeyPair, error) {
	m.mu.RLock()
	if m.keyPair != nil {
		kp := m.keyPair
		m.mu.RUnlock()
		return kp, nil
	}
	keyRing := m.keyRing
	m.mu.RUnlock()

	if keyRing {
		return nil, apperrors.NewInternalf(
			"JWT signing key ring has no active %s signing key", SigningAlgorithmRS256)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock.
	if m.keyPair != nil {
		return m.keyPair, nil
	}

	key, err := rsa.GenerateKey(rand.Reader, m.keySize)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate RSA key")
	}

	kid, err := m.kidFn()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate key ID")
	}

	m.keyPair = &KeyPair{
		Kid:       KidPrefixRSA + kid,
		Algorithm: SigningAlgorithmRS256,
		Key:       key,
	}

	return m.keyPair, nil
}

// GetGMKeyPair returns the current SM2 signing key pair. In key-ring mode a
// missing active SM2-SM3 key fails closed instead of generating one, so every
// replica signs with the same key or refuses to sign at all.
func (m *Manager) GetGMKeyPair() (*GMKeyPair, error) {
	m.mu.RLock()
	if m.gmKeyPair != nil {
		kp := m.gmKeyPair
		m.mu.RUnlock()
		return kp, nil
	}
	keyRing := m.keyRing
	m.mu.RUnlock()

	if keyRing {
		return nil, apperrors.NewInternalf(
			"JWT signing key ring has no active %s signing key", SigningAlgorithmSM2SM3)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.gmKeyPair != nil {
		return m.gmKeyPair, nil
	}

	privateKey, err := platformcrypto.GenerateSM2Key()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate SM2 key")
	}

	kid, err := m.kidFn()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate SM2 key ID")
	}

	m.gmKeyPair = &GMKeyPair{
		Kid:        KidPrefixGM + kid,
		Algorithm:  SigningAlgorithmSM2SM3,
		PrivateKey: privateKey,
		PublicKey:  privateKey.Public(),
	}
	return m.gmKeyPair, nil
}

// JWK represents a JSON Web Key as defined in RFC 7517.
type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// JWKS represents a JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// GetJWKS returns the standard-algorithm public keys a relying party may use to
// verify tokens: the active international signer (RS256, and any ES family) plus
// every retired international key still inside its overlap window. The SM2-SM3
// (国密) family is deliberately excluded so the standard jwks_uri stays parseable
// by strict JOSE libraries (e.g. Nimbus), which reject the unknown SM2 curve and
// would otherwise abort the whole key set. GM keys are published separately by
// GetGMJWKS at /.well-known/jwks-gm.json. Private parameters are never part of JWK.
func (m *Manager) GetJWKS() (*JWKS, error) {
	all, err := m.getAllJWKS()
	if err != nil {
		return nil, err
	}
	keys := make([]JWK, 0, len(all.Keys))
	for i := range all.Keys {
		if !isGMJWK(&all.Keys[i]) {
			keys = append(keys, all.Keys[i])
		}
	}
	return &JWKS{Keys: keys}, nil
}

// GetGMJWKS returns only the SM2-SM3 (国密) public keys: the active GM signer plus
// every retired GM key still inside its overlap window. It is the publication
// source for the dedicated GM endpoint /.well-known/jwks-gm.json, consumed by
// GM-capable relying parties that verify SM2-SM3 tokens by the gm_ kid prefix.
func (m *Manager) GetGMJWKS() (*JWKS, error) {
	all, err := m.getAllJWKS()
	if err != nil {
		return nil, err
	}
	keys := make([]JWK, 0, len(all.Keys))
	for i := range all.Keys {
		if isGMJWK(&all.Keys[i]) {
			keys = append(keys, all.Keys[i])
		}
	}
	return &JWKS{Keys: keys}, nil
}

// isGMJWK classifies a JWK into the 国密 family by its signing algorithm. The
// algorithm is authoritative (kid prefixes are derived from it); SM2-SM3 keys are
// the only family standard JOSE libraries cannot parse.
func isGMJWK(k *JWK) bool {
	return k.Alg == SigningAlgorithmSM2SM3
}

// getAllJWKS builds the full publishable key set across both algorithm families:
// the active signer of each family plus every retired key still inside its
// overlap window (retired_at + overlap). A key dropped from the window is no
// longer published. It fails closed only when no family has any publishable key.
func (m *Manager) getAllJWKS() (*JWKS, error) {
	m.mu.RLock()
	current := m.keyPair
	gmCurrent := m.gmKeyPair
	rsaRetired := append([]*KeyPair(nil), m.retired...)
	gmRetired := append([]*GMKeyPair(nil), m.gmRetired...)
	keyRing := m.keyRing
	now := m.now()
	m.mu.RUnlock()

	// Legacy process-local mode keeps the original behavior: both families are
	// generated on first access and always published.
	if !keyRing {
		var err error
		current, err = m.GetKeyPair()
		if err != nil {
			return nil, err
		}
		gmCurrent, err = m.GetGMKeyPair()
		if err != nil {
			return nil, err
		}
		rsaRetired = nil
		gmRetired = nil
	}

	keys := make([]JWK, 0, 2+len(rsaRetired)+len(gmRetired))
	if current != nil {
		keys = append(keys, *publicKeyToJWK(&current.Key.PublicKey, current.Kid, current.Algorithm))
	}
	for _, kp := range rsaRetired {
		if m.validWithinRetirementWindow(kp.Kid, now) {
			keys = append(keys, *publicKeyToJWK(&kp.Key.PublicKey, kp.Kid, kp.Algorithm))
		}
	}
	if gmCurrent != nil && gmCurrent.PublicKey != nil {
		keys = append(keys, *publicKeyToECJWK(gmCurrent.PublicKey, gmCurrent.Kid, gmCurrent.Algorithm))
	}
	for _, kp := range gmRetired {
		if kp.PublicKey != nil && m.validWithinRetirementWindow(kp.Kid, now) {
			keys = append(keys, *publicKeyToECJWK(kp.PublicKey, kp.Kid, kp.Algorithm))
		}
	}
	if len(keys) == 0 {
		return nil, apperrors.NewInternal("JWT signing key ring has no publishable keys")
	}
	return &JWKS{Keys: keys}, nil
}

// GetKeyPairByID returns the RSA signing key whose kid matches. A verifier must
// call this instead of GetKeyPair: accepting any key regardless of the presented
// kid means an unknown kid is silently verified against the current key, so a
// token minted for a key that was rotated out (ADR-015) still passes.
//
// Rotation keeps retired keys available for verification until their overlap
// window closes; after that the lookup fails like any other unknown kid.
func (m *Manager) GetKeyPairByID(kid string) (*KeyPair, error) {
	if strings.TrimSpace(kid) == "" {
		return nil, apperrors.NewInvalidArgument("token header is missing kid")
	}

	m.mu.RLock()
	retained := append([]*KeyPair(nil), m.retired...)
	current := m.keyPair
	keyRing := m.keyRing
	now := m.now()
	m.mu.RUnlock()

	for _, kp := range retained {
		if kp.Kid == kid && m.validWithinRetirementWindow(kp.Kid, now) {
			return kp, nil
		}
	}

	if current == nil {
		if keyRing {
			// A key ring has no lazily generated keys, so there is nothing to
			// resolve for this family.
			return nil, apperrors.NewUnauthorized("unknown token key id")
		}
		// The signing key has not been generated yet, so no kid can be known.
		if _, err := m.GetKeyPair(); err != nil {
			return nil, err
		}
		m.mu.RLock()
		current = m.keyPair
		m.mu.RUnlock()
	}

	if current == nil || current.Kid != kid {
		return nil, apperrors.NewUnauthorized("unknown token key id")
	}
	return current, nil
}

// GetGMKeyPairByID returns the SM2 key whose kid matches. A gm_ kid must never
// resolve to an RSA key, so this lookup only consults the SM2 set.
func (m *Manager) GetGMKeyPairByID(kid string) (*GMKeyPair, error) {
	if strings.TrimSpace(kid) == "" {
		return nil, apperrors.NewInvalidArgument("token header is missing kid")
	}

	m.mu.RLock()
	retired := append([]*GMKeyPair(nil), m.gmRetired...)
	current := m.gmKeyPair
	keyRing := m.keyRing
	now := m.now()
	m.mu.RUnlock()

	for _, kp := range retired {
		if kp.Kid == kid && m.validWithinRetirementWindow(kp.Kid, now) {
			return kp, nil
		}
	}

	if current == nil {
		if keyRing {
			return nil, apperrors.NewUnauthorized("unknown token key id")
		}
		if _, err := m.GetGMKeyPair(); err != nil {
			return nil, err
		}
		m.mu.RLock()
		current = m.gmKeyPair
		m.mu.RUnlock()
	}

	if current == nil || current.Kid != kid {
		return nil, apperrors.NewUnauthorized("unknown token key id")
	}
	return current, nil
}

// validWithinRetirementWindow reports whether a key may still verify tokens and
// be published. Active and superseded non-retired keys have no expiry; a retired
// key expires at retired_at+overlap. The expiry map is immutable after
// construction, so this read needs no lock.
func (m *Manager) validWithinRetirementWindow(kid string, now time.Time) bool {
	expiry, ok := m.retireExpiry[kid]
	if !ok {
		return true
	}
	return !now.After(expiry)
}

// publicKeyToJWK converts an RSA public key to a JWK.
func publicKeyToJWK(pub *rsa.PublicKey, kid, alg string) *JWK {
	nBytes := pub.N.Bytes()
	eBytes := big.NewInt(int64(pub.E)).Bytes()

	return &JWK{
		Kty: "RSA",
		Use: "sig",
		Kid: kid,
		Alg: alg,
		N:   base64.RawURLEncoding.EncodeToString(nBytes),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}
}

// EncodeRSAPublicKeyDER encodes the public key to DER-encoded PKIX format.
func EncodeRSAPublicKeyDER(pub *rsa.PublicKey) ([]byte, error) {
	return x509.MarshalPKIXPublicKey(pub)
}

// publicKeyToECJWK converts an SM2 public key to an EC JWK (crv=SM2).
func publicKeyToECJWK(pub *platformcrypto.SM2PublicKey, kid, alg string) *JWK {
	x, y, ok := pub.Coordinates()
	if !ok {
		return &JWK{Kty: "EC", Use: "sig", Kid: kid, Alg: alg, Crv: "SM2"}
	}
	return &JWK{
		Kty: "EC",
		Use: "sig",
		Kid: kid,
		Alg: alg,
		Crv: "SM2",
		X:   encodeECCoordinate(x),
		Y:   encodeECCoordinate(y),
	}
}

// encodeECCoordinate renders an EC JWK coordinate as a fixed-length octet
// string. Coordinates below the fixed length are left-padded with zero bytes;
// a valid SM2 coordinate never exceeds it.
func encodeECCoordinate(value *big.Int) string {
	raw := value.Bytes()
	if len(raw) < sm2CoordinateSize {
		padded := make([]byte, sm2CoordinateSize)
		copy(padded[sm2CoordinateSize-len(raw):], raw)
		raw = padded
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func randomKid() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
