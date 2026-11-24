package captchaproviders

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// captchaProviderEncryptedMapper 负责 secret_key 列的 AES-256-GCM 加解密（DB-058 机密列）。
var captchaProviderEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	captchaProviderTable   = "guliangiam.captcha_providers"
	captchaColumnSecretKey = "secret_key"
)

func captchaProviderAAD(providerID string) crypto.AADContext {
	return crypto.AADContext{Table: captchaProviderTable, Column: captchaColumnSecretKey, PKValue: providerID}
}

// rowToDomain 把表行模型转成 domain.CaptchaProvider。SecretKey 若加密则先解密。
func rowToDomain(row captchaProviderRow) (*domain.CaptchaProvider, error) {
	provider := &domain.CaptchaProvider{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		ProviderType:   row.ProviderType,
		SiteKey:        row.SiteKey,
		SecretKey:      row.SecretKey,
		IsEnabled:      row.IsEnabled,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	if err := mysqlshared.UnmarshalJSON(row.Config, &provider.Config); err != nil {
		return nil, err
	}
	if provider.Config == nil {
		provider.Config = map[string]any{}
	}
	if !captchaProviderEncryptedMapper.IsEnabled() {
		return provider, nil
	}
	secret, err := captchaProviderEncryptedMapper.DecryptColumn(captchaProviderAAD(provider.ID), provider.SecretKey)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt captcha provider secret_key")
	}
	provider.SecretKey = secret
	return provider, nil
}

// captchaSecretForWrite 返回持久化机密列所需的值，不改 domain 对象。
func captchaSecretForWrite(provider *domain.CaptchaProvider) (string, error) {
	if !captchaProviderEncryptedMapper.IsEnabled() {
		return provider.SecretKey, nil
	}
	sealed, err := captchaProviderEncryptedMapper.EncryptColumn(captchaProviderAAD(provider.ID), provider.SecretKey)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt captcha provider secret_key")
	}
	return sealed, nil
}
