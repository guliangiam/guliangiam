package providers

import (
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// providers 原生凭据列。这些列在 B1b 之后只允许空串或 envelope 令牌。
const (
	providerTable               = "guliangiam.providers"
	providerColumnClientSecret  = "client_secret"
	providerColumnClientSecret2 = "client_secret2"
	providerColumnPassword      = "password"
	providerColumnAppSecret     = "app_secret"
)

var providerEncryptedMapper = shared.NewEncryptedFieldMapper()

// providerAAD binds a credential column to its provider row (PK guliangiam.providers.id).
func providerAAD(providerID, column string) crypto.AADContext {
	return crypto.AADContext{Table: providerTable, Column: column, PKValue: providerID}
}

// providerSecretValues holds the envelope-encrypted values to persist. It is a
// separate struct so sealProviderSecrets never mutates the caller-owned domain
// object.
type providerSecretValues struct {
	clientSecret  string
	clientSecret2 string
	password      string
	appSecret     string
}

// sealProviderSecrets encrypts the four credential columns with the current
// KEK. Empty values stay empty so "not configured" remains representable.
func sealProviderSecrets(provider *domain.Provider) (providerSecretValues, error) {
	values := providerSecretValues{
		clientSecret:  provider.ClientSecret,
		clientSecret2: provider.ClientSecret2,
		password:      provider.Password,
		appSecret:     provider.AppSecret,
	}
	if !providerEncryptedMapper.IsEnabled() {
		return values, nil
	}

	var err error
	if values.clientSecret, err = providerEncryptedMapper.EncryptColumn(providerAAD(provider.ID, providerColumnClientSecret), provider.ClientSecret); err != nil {
		return providerSecretValues{}, apperrors.WrapInternalf(err, "encrypt %s.%s", providerTable, providerColumnClientSecret)
	}
	if values.clientSecret2, err = providerEncryptedMapper.EncryptColumn(providerAAD(provider.ID, providerColumnClientSecret2), provider.ClientSecret2); err != nil {
		return providerSecretValues{}, apperrors.WrapInternalf(err, "encrypt %s.%s", providerTable, providerColumnClientSecret2)
	}
	if values.password, err = providerEncryptedMapper.EncryptColumn(providerAAD(provider.ID, providerColumnPassword), provider.Password); err != nil {
		return providerSecretValues{}, apperrors.WrapInternalf(err, "encrypt %s.%s", providerTable, providerColumnPassword)
	}
	if values.appSecret, err = providerEncryptedMapper.EncryptColumn(providerAAD(provider.ID, providerColumnAppSecret), provider.AppSecret); err != nil {
		return providerSecretValues{}, apperrors.WrapInternalf(err, "encrypt %s.%s", providerTable, providerColumnAppSecret)
	}
	return values, nil
}

// decryptProviderSecrets opens the four credential columns. A plaintext value
// is a data invariant violation, not a value to use: fail closed and point the
// operator at the backfill command.
func decryptProviderSecrets(provider *domain.Provider) error {
	var err error
	if provider.ClientSecret, err = openProviderSecret(provider.ID, providerColumnClientSecret, provider.ClientSecret); err != nil {
		return err
	}
	if provider.ClientSecret2, err = openProviderSecret(provider.ID, providerColumnClientSecret2, provider.ClientSecret2); err != nil {
		return err
	}
	if provider.Password, err = openProviderSecret(provider.ID, providerColumnPassword, provider.Password); err != nil {
		return err
	}
	if provider.AppSecret, err = openProviderSecret(provider.ID, providerColumnAppSecret, provider.AppSecret); err != nil {
		return err
	}
	return nil
}

func openProviderSecret(providerID, column, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	plaintext, err := providerEncryptedMapper.DecryptEnvelopeColumn(providerAAD(providerID, column), stored)
	if err == nil {
		return plaintext, nil
	}
	if errors.Is(err, crypto.ErrNotEnvelope) {
		return "", apperrors.NewInternalf(
			"%s.%s for provider %s is not envelope-encrypted; run migrate encrypt-provider-secrets",
			providerTable, column, providerID)
	}
	return "", apperrors.WrapInternalf(err, "decrypt %s.%s for provider %s", providerTable, column, providerID)
}

func toProvider(row providerRow) (domain.Provider, error) {
	provider := domain.Provider{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		DisplayName:    row.DisplayName,
		Category:       domain.ProviderCategory(row.Category),
		ProviderType:   domain.ProviderType(row.ProviderType),
		SubType:        row.SubType,
		Method:         row.Method,
		ClientID:       row.ClientID,
		ClientSecret:   row.ClientSecret,
		ClientID2:      row.ClientID2,
		ClientSecret2:  row.ClientSecret2,
		ProviderURL:    row.ProviderURL,
		AuthUrl:        row.AuthUrl,
		TokenUrl:       row.TokenUrl,
		UserInfoUrl:    row.UserInfoUrl,
		CallbackUrl:    row.CallbackUrl,
		SigningAlg:     row.SigningAlg,
		Scopes:         row.Scopes,
		IdPEntityId:    row.IdPEntityId,
		SsoUrl:         row.SsoUrl,
		SloUrl:         row.SloUrl,
		Metadata:       row.Metadata,
		IssuerUrl:      row.IssuerUrl,
		Region:         row.Region,
		Host:           row.Host,
		Port:           row.Port,
		DatabaseType:   row.DatabaseType,
		Template:       row.Template,
		CustomApi:      row.CustomApi,
		CustomHeader:   row.CustomHeader,
		CustomBody:     row.CustomBody,
		Admin:          row.Admin,
		Password:       row.Password,
		AppId:          row.AppId,
		AppSecret:      row.AppSecret,
		AgentID:        row.AgentID,
		ContentType:    row.ContentType,
		Sender:         row.Sender,
		Receiver:       row.Receiver,
		Enabled:        row.Enabled,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
		Family:         domain.Family(row.Family),
		Kind:           domain.Kind(row.Kind),
		Variant:        domain.Variant(row.Variant),
	}
	if err := shared.UnmarshalJSON(row.UserMapping, &provider.UserMapping); err != nil {
		return domain.Provider{}, apperrors.WrapInternalf(err, "unmarshal user_mapping")
	}
	if err := decryptProviderSecrets(&provider); err != nil {
		return domain.Provider{}, err
	}
	return provider, nil
}

func toProviderConfig(row providerConfigRow) (*domain.ProviderConfig, error) {
	config := &domain.ProviderConfig{
		ProviderID: row.ProviderID,
		ConfigType: row.ConfigType,
		UpdatedAt:  row.UpdatedAt,
	}
	if err := shared.UnmarshalJSON(row.Config, &config.Config); err != nil {
		return nil, err
	}
	if config.Config == nil {
		config.Config = map[string]any{}
	}
	return config, nil
}

func toProviderSecretStatus(row providerSecretStatusRow) domain.ProviderSecretStatus {
	return domain.ProviderSecretStatus{
		ID:            row.ID,
		ProviderID:    row.ProviderID,
		SecretType:    row.SecretType,
		Status:        row.Status,
		LastRotatedAt: row.LastRotatedAt,
		ExpiresAt:     row.ExpiresAt,
		UpdatedAt:     row.UpdatedAt,
		Version:       row.Version,
	}
}
