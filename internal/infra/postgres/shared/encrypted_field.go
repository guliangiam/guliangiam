package shared

import (
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// EncryptedFieldMapper re-exposes the process-wide field encryptor for the
// repositories that persist confidential columns.
//
// Every method takes a crypto.AADContext so writes are v2 envelopes bound to
// table/column/primary key and reads can authenticate that binding. There is
// intentionally no unbound Encrypt/Decrypt here: a mapper that forgot the row
// context would otherwise keep producing v1 ciphertext (ADR-015 GM-G7).
//
// The encryptor is resolved lazily on first use rather than at construction:
// DB-058 makes a missing ENCRYPTION_KEY_CURRENT a hard failure, and package
// level mapper variables must not trigger that failure during package init.
type EncryptedFieldMapper struct{}

func NewEncryptedFieldMapper() *EncryptedFieldMapper {
	return &EncryptedFieldMapper{}
}

// EncryptColumn seals a new column value into a v2 envelope. A missing PK
// context fails instead of falling back to v1.
func (m *EncryptedFieldMapper) EncryptColumn(aad crypto.AADContext, value string) (string, error) {
	return crypto.GlobalFieldEncryptor().EncryptColumn(aad, value)
}

// DecryptColumn opens v2 (authenticated with the row AAD), v1 and v0 values.
func (m *EncryptedFieldMapper) DecryptColumn(aad crypto.AADContext, value string) (string, error) {
	return crypto.GlobalFieldEncryptor().DecryptColumn(aad, value)
}

// DecryptEnvelopeColumn refuses legacy tokens and plaintext; use it for
// columns whose values are required to be envelope-encrypted (provider
// credentials). It accepts v1 and v2 and never falls back after a v2
// authentication failure.
func (m *EncryptedFieldMapper) DecryptEnvelopeColumn(aad crypto.AADContext, value string) (string, error) {
	return crypto.GlobalFieldEncryptor().DecryptEnvelopeFieldWithAAD(aad, value)
}

func (m *EncryptedFieldMapper) IsEnabled() bool {
	return crypto.GlobalFieldEncryptor().IsEnabled()
}

// RotateColumn rewrites a value into the v2 generation at the current KEK.
// Returns the re-encrypted value and whether rotation actually occurred.
func (m *EncryptedFieldMapper) RotateColumn(aad crypto.AADContext, value string) (rotated string, changed bool, err error) {
	return crypto.GlobalFieldEncryptor().RotateColumn(aad, value)
}

// HasPreviousKey returns true if a previous encryption key is configured,
// indicating that key rotation is in progress.
func (m *EncryptedFieldMapper) HasPreviousKey() bool {
	return crypto.GlobalFieldEncryptor().HasPreviousKey()
}
