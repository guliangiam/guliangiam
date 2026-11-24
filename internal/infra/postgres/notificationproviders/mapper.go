package notificationproviders

import (
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// notification_providers.access_token 在 B1b 之后只允许空串或 envelope 令牌。
const (
	notificationProviderTable             = "guliangiam.notification_providers"
	notificationProviderColumnAccessToken = "access_token"
)

var notificationProviderEncryptedMapper = shared.NewEncryptedFieldMapper()

func notificationProviderAAD(providerID string) crypto.AADContext {
	return crypto.AADContext{
		Table:   notificationProviderTable,
		Column:  notificationProviderColumnAccessToken,
		PKValue: providerID,
	}
}

// sealAccessToken encrypts the access token for persistence. Empty stays empty
// so "not configured" remains representable, and the caller-owned domain object
// is never mutated.
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

// openAccessToken opens the stored token. A plaintext value is a data
// invariant violation: fail closed and point the operator at the backfill
// command instead of using the value.
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

func toNotificationProvider(row notificationProviderRow) (*domain.NotificationProvider, error) {
	accessToken, err := openAccessToken(row.ID, row.AccessToken)
	if err != nil {
		return nil, err
	}
	provider := &domain.NotificationProvider{
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
	if err := shared.UnmarshalJSON(row.Config, &provider.Config); err != nil {
		return nil, err
	}
	if provider.Config == nil {
		provider.Config = map[string]any{}
	}
	return provider, nil
}
