package kms

import (
	"context"
	"crypto/rand"
	"strconv"
	"strings"
	"sync"
)

// VaultStubAdapter is a contract-level placeholder for HashiCorp Vault KVv2.
//
// Purpose: enable local development of the rotation pipeline (ADR-015)
// before the production Vault adapter lands. It implements KeyManagementService
// in-memory, records all operations to an audit log, and respects the same
// error contract as the real adapter.
//
// This adapter is NOT a security boundary — keys are plaintext in memory and
// can be introspected by tests.
type VaultStubAdapter struct {
	mu     sync.Mutex
	keys   map[string]map[string][]byte // keyID -> version -> plaintext
	audit  []AuditEntry
	closed bool
}

// AuditEntry records every KMS operation. Persisted to guliangiam.records by
// the call site (field_encryption.go). The stub keeps a mirror for unit tests.
type AuditEntry struct {
	Op       string // opResolveLatest / "resolve_version"
	KeyID    string
	Version  string
	Provider string // providerVaultStub
	Status   string // "ok" / "not_found" / "unavailable"
	Reason   string
}

func NewVaultStubAdapter() *VaultStubAdapter {
	return &VaultStubAdapter{
		keys: map[string]map[string][]byte{
			keyIDMaster: {
				"v1": mustRand(32), // pre-arranged key material
			},
		},
	}
}

// PreseedVersion lets tests register key material for a specific version.
func (a *VaultStubAdapter) PreseedVersion(keyID, version string, plaintext []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return
	}
	v, ok := a.keys[keyID]
	if !ok {
		v = map[string][]byte{}
		a.keys[keyID] = v
	}
	v[version] = append([]byte(nil), plaintext...)
}

func (a *VaultStubAdapter) ResolveLatest(_ context.Context, keyID string) (KeyMaterial, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		a.audit = append(a.audit, AuditEntry{Op: opResolveLatest, KeyID: keyID, Provider: providerVaultStub, Status: "unavailable", Reason: "sealed"})
		return KeyMaterial{}, &KeyUnavailableError{Provider: providerVaultStub, Op: opResolveLatest, Reason: "vault sealed"}
	}
	versions, ok := a.keys[keyID]
	if !ok || len(versions) == 0 {
		a.audit = append(a.audit, AuditEntry{Op: opResolveLatest, KeyID: keyID, Provider: providerVaultStub, Status: statusNotFound})
		return KeyMaterial{}, &KeyNotFoundError{KeyID: keyID}
	}
	// Highest version wins; unparsable version keys are ignored rather than
	// silently ranked as version 0.
	best, bestVer := 0, ""
	for ver := range versions {
		n, err := strconv.Atoi(strings.TrimPrefix(ver, "v"))
		if err != nil {
			continue
		}
		if n > best {
			best = n
			bestVer = ver
		}
	}
	if bestVer == "" {
		a.audit = append(a.audit, AuditEntry{Op: opResolveLatest, KeyID: keyID, Provider: providerVaultStub, Status: statusNotFound, Reason: "no parsable version"})
		return KeyMaterial{}, &KeyNotFoundError{KeyID: keyID}
	}
	a.audit = append(a.audit, AuditEntry{Op: opResolveLatest, KeyID: keyID, Version: bestVer, Provider: providerVaultStub, Status: "ok"})
	return KeyMaterial{
		Descriptor: KeyDescriptor{Provider: providerVaultStub, KeyID: keyID, Version: bestVer},
		Plaintext:  append([]byte(nil), versions[bestVer]...),
	}, nil
}

func (a *VaultStubAdapter) ResolveVersion(_ context.Context, keyID, version string) (KeyMaterial, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return KeyMaterial{}, &KeyUnavailableError{Provider: providerVaultStub, Op: opResolveVersion, Reason: "vault sealed"}
	}
	versions, ok := a.keys[keyID]
	if !ok {
		a.audit = append(a.audit, AuditEntry{Op: opResolveVersion, KeyID: keyID, Version: version, Provider: providerVaultStub, Status: statusNotFound})
		return KeyMaterial{}, &KeyNotFoundError{KeyID: keyID, Version: version}
	}
	material, ok := versions[version]
	if !ok {
		a.audit = append(a.audit, AuditEntry{Op: opResolveVersion, KeyID: keyID, Version: version, Provider: providerVaultStub, Status: statusNotFound})
		return KeyMaterial{}, &KeyNotFoundError{KeyID: keyID, Version: version}
	}
	a.audit = append(a.audit, AuditEntry{Op: opResolveVersion, KeyID: keyID, Version: version, Provider: providerVaultStub, Status: "ok"})
	return KeyMaterial{
		Descriptor: KeyDescriptor{Provider: providerVaultStub, KeyID: keyID, Version: version},
		Plaintext:  append([]byte(nil), material...),
	}, nil
}

// Close simulates Vault seal — subsequent operations return KeyUnavailableError.
// Used by the startup-failure test (ADR-009 exit condition 3).
func (a *VaultStubAdapter) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
}

// AuditLog returns a copy of the audit entries recorded so far.
func (a *VaultStubAdapter) AuditLog() []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]AuditEntry, len(a.audit))
	copy(out, a.audit)
	return out
}

var _ KeyManagementService = (*VaultStubAdapter)(nil)

func mustRand(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
