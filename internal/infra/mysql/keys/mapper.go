package keys

import (
	"database/sql"
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// keyEncryptedMapper 负责 keys.private_key 列的信封加解密（DB-058 机密列）。
var keyEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	keyTable            = "guliangiam.keys"
	keyColumnPrivateKey = "private_key"
)

func keyPrivateKeyAAD(keyID string) crypto.AADContext {
	return crypto.AADContext{Table: keyTable, Column: keyColumnPrivateKey, PKValue: keyID}
}

// sealKeyPrivateKey 封存私钥材料。空串表示“无私有材料”，保持空串，
// 写路径据此继续写 SQL NULL，而不是把空串加密成密文。
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

// openKeyPrivateKey 解密私钥材料。R6 之前的明文行 fail-closed（非信封令牌），
// 明确提示先跑通用回填，不得把明文当可用私钥返回。
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

// keyPrivateKeyForWrite 返回持久化值，不改 domain 对象；空值保持 NULL。
func keyPrivateKeyForWrite(k *domain.Key) (sql.NullString, error) {
	if k.PrivateKey == "" {
		return sql.NullString{}, nil
	}
	sealed, err := sealKeyPrivateKey(k.ID, k.PrivateKey)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: sealed, Valid: true}, nil
}
