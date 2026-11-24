package certs

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// certEncryptedMapper 负责 private_key / access_secret 列的 AES-256-GCM 加解密
// （DB-058 机密列）。
var certEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	certTable              = "guliangiam.certs"
	certColumnPrivateKey   = "private_key"
	certColumnAccessSecret = "access_secret"
)

func certAAD(certID, column string) crypto.AADContext {
	return crypto.AADContext{Table: certTable, Column: column, PKValue: certID}
}

// decryptCertSecrets 解密 domain 上的机密列。空串保持空串（表示未配置）。
func decryptCertSecrets(cert *domain.Cert) error {
	if !certEncryptedMapper.IsEnabled() {
		return nil
	}
	privateKey, err := certEncryptedMapper.DecryptColumn(certAAD(cert.ID, certColumnPrivateKey), cert.PrivateKey)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt cert private_key")
	}
	accessSecret, err := certEncryptedMapper.DecryptColumn(certAAD(cert.ID, certColumnAccessSecret), cert.AccessSecret)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt cert access_secret")
	}
	cert.PrivateKey = privateKey
	cert.AccessSecret = accessSecret
	return nil
}

// certSecretsForWrite 返回持久化机密列所需的值，不改 domain 对象。
func certSecretsForWrite(cert *domain.Cert) (privateKey, accessSecret string, err error) {
	if !certEncryptedMapper.IsEnabled() {
		return cert.PrivateKey, cert.AccessSecret, nil
	}
	if privateKey, err = certEncryptedMapper.EncryptColumn(certAAD(cert.ID, certColumnPrivateKey), cert.PrivateKey); err != nil {
		return "", "", apperrors.WrapInternalf(err, "encrypt cert private_key")
	}
	if accessSecret, err = certEncryptedMapper.EncryptColumn(certAAD(cert.ID, certColumnAccessSecret), cert.AccessSecret); err != nil {
		return "", "", apperrors.WrapInternalf(err, "encrypt cert access_secret")
	}
	return privateKey, accessSecret, nil
}
