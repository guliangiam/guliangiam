package notifications

import (
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
)

// NotificationDTO is the data transfer object for notification records.
type NotificationDTO struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	ProviderID     string         `json:"providerId"`
	Recipient      string         `json:"recipient"`
	Subject        string         `json:"subject"`
	Content        string         `json:"content"`
	TemplateID     string         `json:"templateId"`
	Variables      map[string]any `json:"variables"`
	Status         string         `json:"status"`
	Error          string         `json:"error"`
	SentAt         *time.Time     `json:"sentAt"`
	RetryCount     int            `json:"retryCount"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

func toNotificationDTO(n *domain.Notification) *NotificationDTO {
	if n == nil {
		return nil
	}
	return &NotificationDTO{
		ID:             n.ID,
		OrganizationID: n.OrganizationID,
		ProviderID:     n.ProviderID,
		Recipient:      n.Recipient,
		Subject:        n.Subject,
		Content:        n.Content,
		TemplateID:     n.TemplateID,
		Variables:      n.Variables,
		Status:         n.Status,
		Error:          n.Error,
		SentAt:         n.SentAt,
		RetryCount:     n.RetryCount,
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
	}
}
