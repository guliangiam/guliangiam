package keys

import (
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// keyEncryptedMapper seals keys.private_key, the tenant's signing material
// (DB-058 confidential column). New writes are v2 envelopes bound to the key
// row, so rotation can authenticate and rewrite them in place.
var keyEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	keyTable            = "guliangiam.keys"
	keyColumnPrivateKey = "private_key"
)

func keyPrivateKeyAAD(keyID string) crypto.AADContext {
	return crypto.AADContext{Table: keyTable, Column: keyColumnPrivateKey, PKValue: keyID}
}

// sealKeyPrivateKey encrypts private material before it reaches the keys table.
// An empty value means "no private key" and stays empty so Create/Update keep
// writing SQL NULL instead of an encryption of an empty string.
func sealKeyPrivateKey(keyID, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	sealed, err := keyEncryptedMapper.EncryptColumn(keyPrivateKeyAAD(keyID), plaintext)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt keys.private_key for key %s", keyID)
	}
	return sealed, nil
}

// openKeyPrivateKey decrypts stored private material. A pre-GM-R6 plaintext
// value fails closed (non-envelope) and points at the generic backfill instead
// of silently returning the raw key.
func openKeyPrivateKey(keyID, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	plaintext, err := keyEncryptedMapper.DecryptEnvelopeColumn(keyPrivateKeyAAD(keyID), stored)
	if err == nil {
		return plaintext, nil
	}
	if errors.Is(err, crypto.ErrNotEnvelope) {
		return "", apperrors.NewInternalf(
			"guliangiam.keys.private_key for key %s is not envelope-encrypted; run `migrate backfill-encrypted-columns` before reading keys", keyID)
	}
	return "", apperrors.WrapInternalf(err, "decrypt keys.private_key for key %s", keyID)
}

// keyPrivateKeyForWrite returns the value to persist without mutating the
// domain object. NULL semantics are preserved: an empty private key stays nil.
func keyPrivateKeyForWrite(k *domain.Key) (*string, error) {
	if k.PrivateKey == "" {
		return nil, nil
	}
	sealed, err := sealKeyPrivateKey(k.ID, k.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &sealed, nil
}

// scanKey scans a full key row into a domain Key.
func scanKey(sc interface{ Scan(...interface{}) error }) (*domain.Key, error) {
	var k domain.Key
	var storedPrivateKey *string
	var expiresAt *interface{}
	err := sc.Scan(&k.ID, &k.OrganizationID, &k.Name, &k.DisplayName, &k.KeyType, &k.PublicKey, &storedPrivateKey, &k.Algorithm, &expiresAt, &k.IsEnabled, &k.CreatedAt, &k.UpdatedAt, &k.Revision)
	if err != nil {
		return nil, err
	}
	if storedPrivateKey != nil {
		opened, err := openKeyPrivateKey(k.ID, *storedPrivateKey)
		if err != nil {
			return nil, err
		}
		k.PrivateKey = opened
	}
	return &k, nil
}
