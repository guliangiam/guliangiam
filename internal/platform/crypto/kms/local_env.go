package kms

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
)

// LocalEnvAdapter resolves encryption keys from environment variables.
// Mapping (field_encryption.go convention):
//
//	ENCRYPTION_KEY_CURRENT  → key keyIDMaster version "v1" (active)
//	ENCRYPTION_KEY          → key keyIDMaster version "v0" (legacy alias)
//	ENCRYPTION_KEY_PREVIOUS → key keyIDMaster version "v0" (decryption fallback)
//
// Reads go through the platform/config KeyProvider getters (ADR-021 decision 5),
// so key material follows the configured provider and the same TrimSpace
// semantics as the field-encryption path.
//
// This adapter is for development only. It MUST NOT be selected in production —
// Wire composition gates the decision via KMS_BACKEND=local-env / vault-stub.
type LocalEnvAdapter struct {
	// ForceFail — if true, ResolveLatest returns KeyUnavailableError.
	// Used by tests and by the startup path when KMS_BACKEND=invalid.
	ForceFail bool
}

func (a *LocalEnvAdapter) ResolveLatest(_ context.Context, keyID string) (KeyMaterial, error) {
	if a.ForceFail {
		return KeyMaterial{}, &KeyUnavailableError{Provider: providerLocalEnv, Op: opResolveLatest, Reason: reasonForcedFailure}
	}
	if keyID != keyIDMaster {
		return KeyMaterial{}, &KeyNotFoundError{KeyID: keyID}
	}
	// Prefer ENCRYPTION_KEY_CURRENT; fall back to ENCRYPTION_KEY.
	if v := platformconfig.EncryptionKeyCurrent(); v != "" {
		return KeyMaterial{
			Descriptor: KeyDescriptor{Provider: providerLocalEnv, KeyID: keyIDMaster, Version: "v1"},
			Plaintext:  []byte(v),
		}, nil
	}
	if v := platformconfig.EncryptionKeyLegacy(); v != "" {
		return KeyMaterial{
			Descriptor: KeyDescriptor{Provider: providerLocalEnv, KeyID: keyIDMaster, Version: "v0"},
			Plaintext:  []byte(v),
		}, nil
	}
	return KeyMaterial{}, &KeyUnavailableError{Provider: providerLocalEnv, Op: opResolveLatest, Reason: "ENCRYPTION_KEY_CURRENT/ENCRYPTION_KEY not set"}
}

func (a *LocalEnvAdapter) ResolveVersion(_ context.Context, keyID, version string) (KeyMaterial, error) {
	if a.ForceFail {
		return KeyMaterial{}, &KeyUnavailableError{Provider: providerLocalEnv, Op: opResolveVersion, Reason: reasonForcedFailure}
	}
	if keyID != keyIDMaster {
		return KeyMaterial{}, &KeyNotFoundError{KeyID: keyID, Version: version}
	}
	switch version {
	case "v1":
		v := platformconfig.EncryptionKeyCurrent()
		if v == "" {
			return KeyMaterial{}, &KeyNotFoundError{KeyID: keyID, Version: version}
		}
		return KeyMaterial{Descriptor: KeyDescriptor{Provider: providerLocalEnv, KeyID: keyID, Version: version}, Plaintext: []byte(v)}, nil
	case "v0":
		v := platformconfig.EncryptionKeyPrevious()
		if v == "" {
			v = platformconfig.EncryptionKeyLegacy()
		}
		if v == "" {
			return KeyMaterial{}, &KeyNotFoundError{KeyID: keyID, Version: version}
		}
		return KeyMaterial{Descriptor: KeyDescriptor{Provider: providerLocalEnv, KeyID: keyID, Version: version}, Plaintext: []byte(v)}, nil
	default:
		return KeyMaterial{}, &KeyNotFoundError{KeyID: keyID, Version: version}
	}
}

// CurrentVersion reports the highest version known to this adapter (v1 or v0).
// Used by field_encryption.go to drive versioned envelope assignment.
func (a *LocalEnvAdapter) CurrentVersion(_ context.Context, keyID string) (string, error) {
	if a.ForceFail {
		return "", &KeyUnavailableError{Provider: providerLocalEnv, Op: "current_version", Reason: reasonForcedFailure}
	}
	versions := []string{}
	if platformconfig.EncryptionKeyCurrent() != "" {
		versions = append(versions, "v1")
	}
	if platformconfig.EncryptionKeyPrevious() != "" || platformconfig.EncryptionKeyLegacy() != "" {
		versions = append(versions, "v0")
	}
	if len(versions) == 0 {
		return "", &KeyUnavailableError{Provider: providerLocalEnv, Op: "current_version", Reason: "no keys configured"}
	}
	sort.Slice(versions, func(i, j int) bool {
		ai, _ := strconv.Atoi(strings.TrimPrefix(versions[i], "v"))
		aj, _ := strconv.Atoi(strings.TrimPrefix(versions[j], "v"))
		return ai > aj
	})
	return versions[0], nil
}

var _ KeyManagementService = (*LocalEnvAdapter)(nil)

func NewLocalEnvAdapter() *LocalEnvAdapter { return &LocalEnvAdapter{} }

func NewLocalEnvAdapterWithFail(force bool) *LocalEnvAdapter {
	return &LocalEnvAdapter{ForceFail: force}
}

// Compile-time guard — force the error message to surface in go vet if an
// adapter forgets to implement the interface.
var _ = func() error {
	return fmt.Errorf("kms: adapter contract")
}()
