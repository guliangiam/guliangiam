package notifications

import (
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
)

func toNotification(row notificationRow) domain.Notification {
	var subject string
	if row.Subject != nil {
		subject = *row.Subject
	}

	var templateID string
	if row.TemplateID != nil {
		templateID = *row.TemplateID
	}

	var variables map[string]any
	if row.Variables != nil {
		_ = json.Unmarshal(row.Variables, &variables)
	}

	var errMsg string
	if row.Error != nil {
		errMsg = *row.Error
	}

	return domain.Notification{
		ID:                 row.ID,
		OrganizationID:     row.OrganizationID,
		ProviderID:         row.ProviderID,
		UserNotificationID: row.UserNotificationID,
		Channel:            row.Channel,
		ProviderKind:       row.ProviderKind,
		Recipient:          row.Recipient,
		Subject:            subject,
		Content:            row.Content,
		TemplateID:         templateID,
		Variables:          variables,
		Status:             row.Status,
		Error:              errMsg,
		SentAt:             row.SentAt,
		RetryCount:         row.RetryCount,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}
