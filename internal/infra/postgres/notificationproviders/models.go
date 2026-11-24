package notificationproviders

import "time"

type notificationProviderRow struct {
	ID             string
	OrganizationID string
	Name           string
	ProviderType   string
	WebhookURL     string
	AccessToken    string
	Config         []byte
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
