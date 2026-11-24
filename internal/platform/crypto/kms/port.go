// Package kms defines the KeyManagementService port (ADR-009).
//
// Production root keys are hosted by KMS / Vault / Secret Manager; the
// application resolves them via this port at startup. Two adapters ship with
// the repo: LocalEnv (reads ENCRYPTION_KEY_* env vars, dev only) and
// VaultStub (contract-level placeholder for HashiCorp Vault integration).
//
// Design notes (ADR-009):
//   - The port must not leak KMS-provider types into the domain.
//   - Unreachable KMS → reject startup (no plaintext fallback).
//   - Keys are identified by version; rotation emits audit records.
package kms

import (
	"context"
	"fmt"
)

const (
	// opResolveVersion 是「解析指定密钥版本」的 Op 标识，同时用于错误体与审计记录。
	opResolveVersion = "resolve_version"
	// opResolveLatest 是「解析当前最新版本」的 Op 标识。
	opResolveLatest = "resolve_latest"
	// reasonForcedFailure 是本地适配器强制失败分支的原因文案。
	reasonForcedFailure = "forced failure"
	// providerLocalEnv 是本地环境变量适配器的 provider 名。
	providerLocalEnv = "local-env"
	// providerVaultStub 是 Vault 契约级占位适配器的 provider 名。
	providerVaultStub = "vault-stub"
	// keyIDMaster 是根密钥的逻辑名。
	keyIDMaster = "master"
	// statusNotFound 是审计记录中「密钥版本不存在」的状态值。
	statusNotFound = "not_found"
)

// KeyDescriptor identifies a managed encryption key by provider, key id, and
// version. The provider field selects the KMS backend (local-env / vault /
// aws-kms) — stubs are limited to local-env and vault-stub.
type KeyDescriptor struct {
	Provider string // "local-env", "vault-stub", "aws-kms", ...
	KeyID    string // logical key name in the provider
	Version  string // version string ("v1", "v2", ...)
}

// KeyMaterial holds resolved ciphertext / plaintext key bytes along with the
// descriptor they came from. Callers must not log or persist the plaintext.
type KeyMaterial struct {
	Descriptor KeyDescriptor
	Plaintext  []byte // DEK/KEK raw bytes — zero after use
}

// KeyManagementService is the application-facing KMS abstraction (ADR-009).
//
// Two adapters are required for [Current]:
//   - LocalEnvAdapter — reads ENCRYPTION_KEY_CURRENT / ENCRYPTION_KEY_PREVIOUS
//     env vars. For development only; must never run in production.
//   - VaultStubAdapter — contract-level placeholder for HashiCorp Vault KVv2.
//     Emits audit events and returns pre-arranged key material without a real
//     Vault cluster. Enables local development of the rotation pipeline
//     (ADR-015) before the production Vault adapter lands.
type KeyManagementService interface {
	// ResolveLatest returns the active version of the key identified by id.
	// Implementations MUST reject (not silently fall back) when the backend
	// is unreachable or the key id does not exist — ADR-009 exit condition 3.
	ResolveLatest(ctx context.Context, keyID string) (KeyMaterial, error)

	// ResolveVersion returns a specific historical version. Used during
	// rotation replay and batch re-encryption (ADR-015).
	ResolveVersion(ctx context.Context, keyID, version string) (KeyMaterial, error)
}

// KeyNotFoundError is returned when the requested key id or version does not
// exist in the KMS backend.
type KeyNotFoundError struct {
	KeyID   string
	Version string
}

func (e *KeyNotFoundError) Error() string {
	if e.Version == "" {
		return fmt.Sprintf("kms: key %q not found", e.KeyID)
	}
	return fmt.Sprintf("kms: key %q version %q not found", e.KeyID, e.Version)
}

// KeyUnavailableError is returned when the KMS backend is unreachable. Callers
// must fail closed (reject startup / return 503) rather than fall back to
// plaintext storage — ADR-009 exit condition 3.
type KeyUnavailableError struct {
	Provider string
	Op       string // "resolve_latest" / "resolve_version"
	Reason   string
}

func (e *KeyUnavailableError) Error() string {
	return fmt.Sprintf("kms: backend %q unavailable during %s: %s", e.Provider, e.Op, e.Reason)
}
