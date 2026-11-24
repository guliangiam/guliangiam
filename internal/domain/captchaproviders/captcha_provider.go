package captchaproviders

import "time"

type CaptchaProvider struct {
	ID             string
	OrganizationID string
	Name           string
	ProviderType   string
	SiteKey        string
	SecretKey      string
	Config         map[string]any
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
