package notificationproviders

import (
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// notification_providers.access_token 在 B1b 之后只允许空串或 envelope 令牌。
const (
	notificationProviderTable             = "guliangiam.notification_providers"
	notificationProviderColumnAccessToken = "access_token"
)

var notificationProviderEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

func notificationProviderAAD(providerID string) crypto.AADContext {
	return crypto.AADContext{
		Table:   notificationProviderTable,
		Column:  notificationProviderColumnAccessToken,
		PKValue: providerID,
	}
}

// sealAccessToken 加密 access_token 写值；空串保持空串，且不修改调用方对象。
func sealAccessToken(provider *domain.NotificationProvider) (string, error) {
	if !notificationProviderEncryptedMapper.IsEnabled() {
		return provider.AccessToken, nil
	}
	sealed, err := notificationProviderEncryptedMapper.EncryptColumn(notificationProviderAAD(provider.ID), provider.AccessToken)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt %s.%s", notificationProviderTable, notificationProviderColumnAccessToken)
	}
	return sealed, nil
}

// openAccessToken 只接受 envelope 令牌；历史明文按数据不变式错误处理。
func openAccessToken(providerID, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	plaintext, err := notificationProviderEncryptedMapper.DecryptEnvelopeColumn(notificationProviderAAD(providerID), stored)
	if err == nil {
		return plaintext, nil
	}
	if errors.Is(err, crypto.ErrNotEnvelope) {
		return "", apperrors.NewInternalf(
			"%s.%s for notification provider %s is not envelope-encrypted; run migrate encrypt-provider-secrets",
			notificationProviderTable, notificationProviderColumnAccessToken, providerID)
	}
	return "", apperrors.WrapInternalf(err, "decrypt %s.%s for notification provider %s",
		notificationProviderTable, notificationProviderColumnAccessToken, providerID)
}

// rowToDomain 把表行模型转成 domain.NotificationProvider。
func rowToDomain(row notificationProviderRow) (*domain.NotificationProvider, error) {
	accessToken, err := openAccessToken(row.ID, row.AccessToken)
	if err != nil {
		return nil, err
	}
	p := &domain.NotificationProvider{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		ProviderType:   row.ProviderType,
		WebhookURL:     row.WebhookURL,
		AccessToken:    accessToken,
		IsEnabled:      row.IsEnabled,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	if err := mysqlshared.UnmarshalJSON(row.Config, &p.Config); err != nil {
		return nil, err
	}
	if p.Config == nil {
		p.Config = map[string]any{}
	}
	return p, nil
}
