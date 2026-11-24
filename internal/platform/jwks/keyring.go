package jwks

import (
	"crypto/rsa"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// DefaultRetirementOverlap is the verification/publishing window kept for a
	// retired signing key. It matches the lifecycle policy's 2x 30-day maximum
	// token lifetime, providing a conservative window even for opaque tokens.
	DefaultRetirementOverlap = 60 * 24 * time.Hour

	// MinRetirementOverlap is the floor accepted from a key ring file. A
	// configured window shorter than the lifecycle policy is rejected to avoid
	// expiring retired signing keys while tokens may still be valid.
	MinRetirementOverlap = DefaultRetirementOverlap
)

// keyRingFile is the deployment-facing JSON document. `overlap` is optional so
// the common case only carries keys; when present it must be a Go duration
// string ("1440h", "1500h").
type keyRingFile struct {
	Overlap string             `json:"overlap,omitempty"`
	Keys    []keyRingFileEntry `json:"keys"`
}

type keyRingFileEntry struct {
	Kid           string     `json:"kid"`
	Algorithm     string     `json:"algorithm"`
	PrivateKeyPEM string     `json:"private_key_pem"`
	CreatedAt     *time.Time `json:"created_at"`
	RetiredAt     *time.Time `json:"retired_at"`
}

type keyRingKey struct {
	kid       string
	algorithm string
	createdAt time.Time
	retiredAt *time.Time
	active    bool
	rsa       *rsa.PrivateKey
	gm        *platformcrypto.SM2PrivateKey
}

// KeyRing is an immutable set of deployment-provided JWT signing keys. It is
// built by LoadKeyRing or ParseKeyRing and injected into Manager through
// NewManagerWithKeyRing.
type KeyRing struct {
	overlap time.Duration
	keys    []keyRingKey
}

// LoadKeyRing reads and validates a key ring JSON file. An unreadable file or
// invalid document is a startup error: the caller must not fall back to
// process-local keys.
func LoadKeyRing(path string) (*KeyRing, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, apperrors.NewInvalidArgument("JWT signing key ring path is empty")
	}
	data, err := os.ReadFile(trimmed)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read JWT signing key ring file %q", trimmed)
	}
	return ParseKeyRing(data)
}

// ParseKeyRing validates an in-memory key ring document.
//
// Invariants enforced here (fail-close, no silent defaults for invalid input):
//   - kid is globally unique and its prefix matches the algorithm
//     (rsa_ for RS256, gm_ for SM2-SM3);
//   - algorithm is one of the two algorithms this manager can actually sign
//     with (RS256, SM2-SM3), not merely the wider JOSE verification allow-list;
//   - created_at is required and retired_at, when present, is not earlier;
//   - private key material parses for the declared algorithm;
//   - every algorithm family present in the file has exactly one active signer:
//     the newest entry whose retired_at is null. Same-family entries are
//     verification keys (retired or superseded) and are published/verifiable
//     according to the overlap window.
func ParseKeyRing(data []byte) (*KeyRing, error) {
	var file keyRingFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, apperrors.WrapInvalidArgumentf(err, "parse JWT signing key ring JSON")
	}

	overlap := DefaultRetirementOverlap
	if raw := strings.TrimSpace(file.Overlap); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return nil, apperrors.WrapInvalidArgumentf(err, "parse JWT signing key ring overlap %q", raw)
		}
		if parsed < MinRetirementOverlap {
			return nil, apperrors.NewInvalidArgumentf(
				"JWT signing key ring overlap %s is shorter than the %s minimum (2x the 30-day maximum token lifetime)",
				parsed, MinRetirementOverlap)
		}
		overlap = parsed
	}

	if len(file.Keys) == 0 {
		return nil, apperrors.NewInvalidArgument("JWT signing key ring must contain at least one key")
	}

	keys := make([]keyRingKey, 0, len(file.Keys))
	seen := make(map[string]struct{}, len(file.Keys))
	for i, entry := range file.Keys {
		parsed, err := parseKeyRingEntry(i, entry)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[parsed.kid]; exists {
			return nil, apperrors.NewInvalidArgumentf(
				"JWT signing key ring key %d: duplicate kid %q", i, parsed.kid)
		}
		seen[parsed.kid] = struct{}{}
		keys = append(keys, parsed)
	}

	if err := selectActiveKeyRingKeys(keys); err != nil {
		return nil, err
	}

	// Deterministic publication order: RSA family first (matching the legacy
	// JWKS order), the active signer before verification keys, newest first.
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].algorithm != keys[j].algorithm {
			return keys[i].algorithm == SigningAlgorithmRS256
		}
		if keys[i].active != keys[j].active {
			return keys[i].active
		}
		if !keys[i].createdAt.Equal(keys[j].createdAt) {
			return keys[i].createdAt.After(keys[j].createdAt)
		}
		return keys[i].kid < keys[j].kid
	})

	return &KeyRing{overlap: overlap, keys: keys}, nil
}

func parseKeyRingEntry(index int, entry keyRingFileEntry) (keyRingKey, error) {
	kid := strings.TrimSpace(entry.Kid)
	if kid == "" {
		return keyRingKey{}, apperrors.NewInvalidArgumentf(
			"JWT signing key ring key %d: kid is required", index)
	}

	algorithm := strings.TrimSpace(entry.Algorithm)
	kidPrefix, supported := kidPrefixForAlgorithm(algorithm)
	if !supported {
		return keyRingKey{}, apperrors.NewInvalidArgumentf(
			"JWT signing key ring key %q: algorithm %q is not supported; supported: %s, %s",
			kid, algorithm, SigningAlgorithmRS256, SigningAlgorithmSM2SM3)
	}
	if !strings.HasPrefix(kid, kidPrefix) || len(kid) == len(kidPrefix) {
		return keyRingKey{}, apperrors.NewInvalidArgumentf(
			"JWT signing key ring key %q: kid must start with %q for algorithm %s",
			kid, kidPrefix, algorithm)
	}

	if entry.CreatedAt == nil || entry.CreatedAt.IsZero() {
		return keyRingKey{}, apperrors.NewInvalidArgumentf(
			"JWT signing key ring key %q: created_at is required", kid)
	}
	createdAt := entry.CreatedAt.UTC()

	var retiredAt *time.Time
	if entry.RetiredAt != nil {
		retired := entry.RetiredAt.UTC()
		if retired.Before(createdAt) {
			return keyRingKey{}, apperrors.NewInvalidArgumentf(
				"JWT signing key ring key %q: retired_at %s is earlier than created_at %s",
				kid, retired.Format(time.RFC3339), createdAt.Format(time.RFC3339))
		}
		retiredAt = &retired
	}

	if strings.TrimSpace(entry.PrivateKeyPEM) == "" {
		return keyRingKey{}, apperrors.NewInvalidArgumentf(
			"JWT signing key ring key %q: private_key_pem is required", kid)
	}

	parsed := keyRingKey{
		kid:       kid,
		algorithm: algorithm,
		createdAt: createdAt,
		retiredAt: retiredAt,
	}
	switch algorithm {
	case SigningAlgorithmRS256:
		key, err := platformcrypto.ParseRSAPrivateKeyPEM(entry.PrivateKeyPEM)
		if err != nil {
			return keyRingKey{}, apperrors.WrapInvalidArgumentf(
				err, "JWT signing key ring key %q: parse RSA private key", kid)
		}
		if err := key.Validate(); err != nil {
			return keyRingKey{}, apperrors.WrapInvalidArgumentf(
				err, "JWT signing key ring key %q: invalid RSA private key", kid)
		}
		parsed.rsa = key
	case SigningAlgorithmSM2SM3:
		key, err := platformcrypto.ParseSM2PrivateKeyPEM(entry.PrivateKeyPEM)
		if err != nil {
			return keyRingKey{}, apperrors.WrapInvalidArgumentf(
				err, "JWT signing key ring key %q: parse SM2 private key", kid)
		}
		if key.Public() == nil {
			return keyRingKey{}, apperrors.NewInvalidArgumentf(
				"JWT signing key ring key %q: SM2 private key has no public key", kid)
		}
		parsed.gm = key
	}
	return parsed, nil
}

// selectActiveKeyRingKeys marks the unique signer per family: the newest entry
// whose retired_at is null. A family whose entries are all retired is rejected
// because it would have no usable signer; a family absent from the file is
// allowed (signing that family then fails closed in Manager).
func selectActiveKeyRingKeys(keys []keyRingKey) error {
	active := make(map[string]int, 2)
	present := make(map[string]struct{}, 2)
	for i := range keys {
		algorithm := keys[i].algorithm
		present[algorithm] = struct{}{}
		if keys[i].retiredAt != nil {
			continue
		}
		existing, ok := active[algorithm]
		if !ok {
			active[algorithm] = i
			continue
		}
		switch {
		case keys[i].createdAt.After(keys[existing].createdAt):
			active[algorithm] = i
		case keys[i].createdAt.Equal(keys[existing].createdAt):
			return apperrors.NewInvalidArgumentf(
				"JWT signing key ring has two active %s keys with the same created_at; the signer would be ambiguous",
				algorithm)
		}
	}
	for algorithm := range present {
		index, ok := active[algorithm]
		if !ok {
			return apperrors.NewInvalidArgumentf(
				"JWT signing key ring has no active %s key (every key has retired_at set)", algorithm)
		}
		keys[index].active = true
	}
	return nil
}

// Overlap returns the verification/publishing window applied to retired keys.
func (r *KeyRing) Overlap() time.Duration {
	if r == nil {
		return 0
	}
	return r.overlap
}

// Len returns the number of keys in the ring.
func (r *KeyRing) Len() int {
	if r == nil {
		return 0
	}
	return len(r.keys)
}

// ActiveKids returns the kid of the unique signer for every algorithm family
// present. It is safe to log: kid is public key identifier, not key material.
func (r *KeyRing) ActiveKids() []string {
	if r == nil {
		return nil
	}
	kids := make([]string, 0, 2)
	for _, key := range r.keys {
		if key.active {
			kids = append(kids, key.kid)
		}
	}
	return kids
}
