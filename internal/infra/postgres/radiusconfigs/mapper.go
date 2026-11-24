package radiusconfigs

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// radiusConfigEncryptedMapper guards the DB-058 confidential column:
// shared_secret is stored as AES-256-GCM ciphertext.
var radiusConfigEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	radiusConfigTable        = "guliangiam.radius_configs"
	radiusColumnSharedSecret = "shared_secret"
)

func radiusConfigAAD(configID string) crypto.AADContext {
	return crypto.AADContext{Table: radiusConfigTable, Column: radiusColumnSharedSecret, PKValue: configID}
}

func toRadiusConfig(row radiusConfigRow) (*domain.RadiusConfig, error) {
	config := &domain.RadiusConfig{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		Host:           row.Host,
		Port:           row.Port,
		SharedSecret:   row.SharedSecret,
		AuthProtocol:   row.AuthProtocol,
		IsEnabled:      row.IsEnabled,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	if !radiusConfigEncryptedMapper.IsEnabled() {
		return config, nil
	}
	secret, err := radiusConfigEncryptedMapper.DecryptColumn(radiusConfigAAD(config.ID), config.SharedSecret)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt radius config shared_secret")
	}
	config.SharedSecret = secret
	return config, nil
}

// radiusSecretForWrite returns the value to persist for the confidential column
// without mutating the domain object.
func radiusSecretForWrite(config *domain.RadiusConfig) (string, error) {
	if !radiusConfigEncryptedMapper.IsEnabled() {
		return config.SharedSecret, nil
	}
	sealed, err := radiusConfigEncryptedMapper.EncryptColumn(radiusConfigAAD(config.ID), config.SharedSecret)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt radius config shared_secret")
	}
	return sealed, nil
}
