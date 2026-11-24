package webhooks

import (
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
)

// toWebhook 把 MySQL 表扫描结果转成 domain.Webhook。
// events 从 JSON 反序列化为 []string；secret 可选加密/解密（EncryptedFieldMapper）。
func toWebhook(row webhookRow) (*domain.Webhook, error) {
	webhook := &domain.Webhook{
		ID:                    row.ID,
		OrganizationID:        row.OrganizationID,
		Name:                  row.Name,
		DisplayName:           row.DisplayName,
		URL:                   row.URL,
		Method:                row.Method,
		Headers:               row.Headers,
		IsEnabled:             row.IsEnabled,
		Secret:                row.Secret,
		RetryCount:            row.RetryCount,
		TimeoutMs:             row.TimeoutMs,
		Organization:          row.Organization,
		ContentType:           row.ContentType,
		CustomBody:            row.CustomBody,
		CustomHeaders:         row.CustomHeaders,
		CertID:                row.CertID,
		IsError:               row.IsError,
		ErrorText:             row.ErrorText,
		IsUserExtended:        row.IsUserExtended,
		SingleOrgOnly:         row.SingleOrgOnly,
		MaxRetries:            row.MaxRetries,
		RetryInterval:         row.RetryInterval,
		UseExponentialBackoff: row.UseExponentialBackoff,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
		Revision:              row.Revision,
	}
	if err := mysqlshared.UnmarshalJSON(row.Events, &webhook.Events); err != nil {
		return nil, err
	}
	if webhook.Events == nil {
		webhook.Events = []string{}
	}
	if err := mysqlshared.UnmarshalJSON(row.TokenFields, &webhook.TokenFields); err != nil {
		return nil, err
	}
	if webhook.TokenFields == nil {
		webhook.TokenFields = []string{}
	}
	if err := mysqlshared.UnmarshalJSON(row.ObjectFields, &webhook.ObjectFields); err != nil {
		return nil, err
	}
	if webhook.ObjectFields == nil {
		webhook.ObjectFields = []string{}
	}
	if err := decryptWebhookSecrets(webhook); err != nil {
		return nil, err
	}
	return webhook, nil
}

var webhookEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	webhookTable        = "guliangiam.webhooks"
	webhookColumnSecret = "secret"
)

func webhookAAD(webhookID string) crypto.AADContext {
	return crypto.AADContext{Table: webhookTable, Column: webhookColumnSecret, PKValue: webhookID}
}

func encryptWebhookSecrets(w *domain.Webhook) error {
	if !webhookEncryptedMapper.IsEnabled() {
		return nil
	}
	enc, err := webhookEncryptedMapper.EncryptColumn(webhookAAD(w.ID), w.Secret)
	if err != nil {
		return apperrors.WrapInternalf(err, "encrypt secret")
	}
	w.Secret = enc
	return nil
}

func decryptWebhookSecrets(w *domain.Webhook) error {
	if !webhookEncryptedMapper.IsEnabled() {
		return nil
	}
	dec, err := webhookEncryptedMapper.DecryptColumn(webhookAAD(w.ID), w.Secret)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt secret")
	}
	w.Secret = dec
	return nil
}
