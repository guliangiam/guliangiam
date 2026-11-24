package jwks

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// EphemeralSigningPolicy is the startup decision for process-local signing keys.
type EphemeralSigningPolicy struct {
	// Allowed reports whether the manager may generate keys at runtime.
	Allowed bool
	// Escaped reports that production explicitly opted in through
	// AllowEphemeralSigningKeysEnv.
	Escaped bool
}

// ResolveEphemeralSigningPolicy is the single decision point for whether a
// process may mint JWT signing keys locally. A configured key ring always wins
// (Allowed=false); its load errors stay startup errors rather than falling back
// to generated keys. Without a key ring, production fails closed unless the
// escape hatch is explicitly enabled, while every other environment keeps the
// historical ephemeral behavior.
func ResolveEphemeralSigningPolicy(isProduction bool, keyRingPath, allowEphemeral string) (EphemeralSigningPolicy, error) {
	if strings.TrimSpace(keyRingPath) != "" {
		return EphemeralSigningPolicy{}, nil
	}
	if !isProduction {
		return EphemeralSigningPolicy{Allowed: true}, nil
	}
	if truthyEnvValue(allowEphemeral) {
		return EphemeralSigningPolicy{Allowed: true, Escaped: true}, nil
	}
	return EphemeralSigningPolicy{}, apperrors.NewInvalidArgument(
		"APP_ENV=production requires JWT_SIGNING_KEYS_FILE; refusing to start with process-local ephemeral signing keys: " +
			"replicas would not share keys and every restart would change kid. " +
			"Provide the key ring file, or set ALLOW_EPHEMERAL_SIGNING_KEYS=true (also 1/yes/on) " +
			"only for a single-replica throwaway deployment")
}

func truthyEnvValue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
