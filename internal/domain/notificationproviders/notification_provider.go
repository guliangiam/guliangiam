package notificationproviders

import "time"

type NotificationProvider struct {
	ID             string
	OrganizationID string
	Name           string
	ProviderType   string
	WebhookURL     string
	AccessToken    string
	Config         map[string]any
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
