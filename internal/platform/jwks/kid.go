package jwks

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// KidPrefixRSA and KidPrefixGM are the public kid namespaces for the two
// signing algorithm families. Every published JWK and every signed JWT header
// carries exactly one of them, so a relying party can select the verification
// key family from the kid alone (AC-CRYPTO-002).
const (
	KidPrefixRSA = "rsa_"
	KidPrefixGM  = "gm_"
)

// kidPrefixForAlgorithm returns the kid prefix that belongs to algorithm.
func kidPrefixForAlgorithm(algorithm string) (string, bool) {
	switch algorithm {
	case SigningAlgorithmRS256:
		return KidPrefixRSA, true
	case SigningAlgorithmSM2SM3:
		return KidPrefixGM, true
	default:
		return "", false
	}
}

// KidForAlgorithm derives the kid for a key identified by id under the given
// signing algorithm. The derivation is deterministic and family-scoped:
//
//   - RS256 ids always start with rsa_, SM2-SM3 ids with gm_;
//   - an id that already carries its own family prefix is returned unchanged,
//     so deployment key-ring kids and application key ids follow one rule;
//   - an id carrying the other family's prefix is still re-prefixed, so an id
//     that looks like a gm_ kid can never be published as an RSA kid.
//
// The same id under different algorithms therefore yields different kids, and
// the same public key published by the platform and by an application keeps a
// family-consistent kid at every endpoint.
func KidForAlgorithm(algorithm, id string) (string, error) {
	prefix, ok := kidPrefixForAlgorithm(algorithm)
	if !ok {
		return "", apperrors.NewInvalidArgumentf("unsupported signing algorithm %q", algorithm)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", apperrors.NewInvalidArgument("key id is required")
	}
	if strings.HasPrefix(id, prefix) {
		return id, nil
	}
	return prefix + id, nil
}
