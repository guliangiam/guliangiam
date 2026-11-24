package captchaproviders

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// captchaProviderEncryptedMapper guards the DB-058 confidential column:
// secret_key is stored as AES-256-GCM ciphertext.
var captchaProviderEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	captchaProviderTable   = "guliangiam.captcha_providers"
	captchaColumnSecretKey = "secret_key"
)

func captchaProviderAAD(providerID string) crypto.AADContext {
	return crypto.AADContext{Table: captchaProviderTable, Column: captchaColumnSecretKey, PKValue: providerID}
}

func toCaptchaProvider(row captchaProviderRow) (*domain.CaptchaProvider, error) {
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
	if err := shared.UnmarshalJSON(row.Config, &provider.Config); err != nil {
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

// captchaSecretForWrite returns the value to persist for the confidential
// column without mutating the domain object.
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
