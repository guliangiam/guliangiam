package bootstrap

import (
	"log/slog"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

const jwtSigningKeySize = 2048

// newJWKSManager builds the JWT signing-key manager from an optional key ring
// file. Without one, whether ephemeral keys are acceptable is decided only by
// platformjwks.ResolveEphemeralSigningPolicy: production refuses to start
// unless ALLOW_EPHEMERAL_SIGNING_KEYS explicitly opts in, while other
// environments keep the legacy process-local behavior with a warning. A
// configured path is authoritative and fail-close; there is no silent fallback
// to generated keys.
func newJWKSManager(keyRingPath string) (*platformjwks.Manager, error) {
	path := strings.TrimSpace(keyRingPath)
	policy, err := platformjwks.ResolveEphemeralSigningPolicy(
		config.IsProduction(), path, envOrEmpty("ALLOW_EPHEMERAL_SIGNING_KEYS"))
	if err != nil {
		return nil, apperrors.WrapInternal(err, "resolve JWT signing key policy")
	}

	if policy.Allowed {
		if policy.Escaped {
			slog.Warn("ephemeral signing keys explicitly allowed in production by ALLOW_EPHEMERAL_SIGNING_KEYS: " +
				"replicas do not share JWT signing keys and every restart changes kid")
		} else {
			slog.Warn("ephemeral signing keys: JWT_SIGNING_KEYS_FILE is not set; " +
				"each process and restart generates different JWT signing keys, so multiple replicas cannot verify each other's tokens")
		}
		return platformjwks.NewManager(jwtSigningKeySize, nil), nil
	}

	ring, err := platformjwks.LoadKeyRing(path)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "load JWT signing key ring")
	}
	manager, err := platformjwks.NewManagerWithKeyRing(ring)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "initialize JWT signing key ring")
	}

	// kid and overlap are public metadata; private key material is never logged.
	slog.Info("JWT signing keys loaded from key ring",
		"path", path,
		"keys", ring.Len(),
		"active_kids", ring.ActiveKids(),
		"overlap", ring.Overlap().String(),
	)
	return manager, nil
}
