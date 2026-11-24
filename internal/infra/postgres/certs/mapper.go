package certs

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// certEncryptedMapper guards the DB-058 confidential columns: private_key and
// access_secret are stored as AES-256-GCM ciphertext.
var certEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	certTable              = "guliangiam.certs"
	certColumnPrivateKey   = "private_key"
	certColumnAccessSecret = "access_secret"
)

func certAAD(certID, column string) crypto.AADContext {
	return crypto.AADContext{Table: certTable, Column: column, PKValue: certID}
}

func toCert(row certRow) (*domain.Cert, error) {
	cert := &domain.Cert{
		ID:                     row.ID,
		OrganizationID:         row.OrganizationID,
		Name:                   row.Name,
		DisplayName:            row.DisplayName,
		Scope:                  row.Scope,
		Type:                   row.Type,
		CryptoAlgorithm:        row.CryptoAlgorithm,
		BitSize:                row.BitSize,
		ExpireInYears:          row.ExpireInYears,
		ExpireTime:             row.ExpireTime,
		Certificate:            row.Certificate,
		PrivateKey:             row.PrivateKey,
		AuthorityPublicKey:     row.AuthorityPublicKey,
		AuthorityRootPublicKey: row.AuthorityRootPublicKey,
		AcmeIssuer:             row.AcmeIssuer,
		AcmeEndpoint:           row.AcmeEndpoint,
		Provider:               row.Provider,
		Account:                row.Account,
		AccessKey:              row.AccessKey,
		AccessSecret:           row.AccessSecret,
		DomainExpireTime:       row.DomainExpireTime,
		CreatedAt:              row.CreatedAt,
		UpdatedAt:              row.UpdatedAt,
		Revision:               row.Revision,
	}
	if err := shared.UnmarshalJSON(row.Domain, &cert.Domain); err != nil {
		return nil, err
	}
	if cert.Domain == nil {
		cert.Domain = []string{}
	}
	if !certEncryptedMapper.IsEnabled() {
		return cert, nil
	}
	privateKey, err := certEncryptedMapper.DecryptColumn(certAAD(cert.ID, certColumnPrivateKey), cert.PrivateKey)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt cert private_key")
	}
	accessSecret, err := certEncryptedMapper.DecryptColumn(certAAD(cert.ID, certColumnAccessSecret), cert.AccessSecret)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt cert access_secret")
	}
	cert.PrivateKey = privateKey
	cert.AccessSecret = accessSecret
	return cert, nil
}

// certSecretsForWrite returns the values to persist for the confidential
// columns without mutating the domain object.
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
