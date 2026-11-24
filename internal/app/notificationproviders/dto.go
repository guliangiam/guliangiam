package notificationproviders

import "time"

type NotificationProviderListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	ProviderType   string    `json:"providerType"`
	WebhookURL     string    `json:"webhookUrl"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type NotificationProviderDetail struct {
	NotificationProviderListItem
	AccessTokenSet    bool           `json:"accessTokenSet"`
	AccessTokenMasked string         `json:"accessTokenMasked,omitempty"`
	Config            map[string]any `json:"config"`
	Revision          int64          `json:"revision"`
}
