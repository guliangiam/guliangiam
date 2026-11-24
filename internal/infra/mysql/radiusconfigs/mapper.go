package radiusconfigs

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// radiusConfigEncryptedMapper 负责 shared_secret 列的 AES-256-GCM 加解密（DB-058 机密列）。
var radiusConfigEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	radiusConfigTable        = "guliangiam.radius_configs"
	radiusColumnSharedSecret = "shared_secret"
)

func radiusConfigAAD(configID string) crypto.AADContext {
	return crypto.AADContext{Table: radiusConfigTable, Column: radiusColumnSharedSecret, PKValue: configID}
}

// rowToDomain 把表行模型转成 domain.RadiusConfig。SharedSecret 若加密则先解密。
// radiusconfigs 没有 JSON 字段，直接字段赋值即可。
func rowToDomain(row radiusConfigRow) (*domain.RadiusConfig, error) {
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

// radiusSecretForWrite 返回持久化机密列所需的值，不改 domain 对象。
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
