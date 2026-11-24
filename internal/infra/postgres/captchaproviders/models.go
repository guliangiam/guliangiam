package captchaproviders

import "time"

type captchaProviderRow struct {
	ID             string
	OrganizationID string
	Name           string
	ProviderType   string
	SiteKey        string
	SecretKey      string
	Config         []byte
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
