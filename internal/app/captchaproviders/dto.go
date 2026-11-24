package captchaproviders

import "time"

type CaptchaProviderListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	ProviderType   string    `json:"providerType"`
	SiteKey        string    `json:"siteKey"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type CaptchaProviderDetail struct {
	CaptchaProviderListItem
	SecretKeySet    bool           `json:"secretKeySet"`
	SecretKeyMasked string         `json:"secretKeyMasked,omitempty"`
	Config          map[string]any `json:"config"`
	Revision        int64          `json:"revision"`
}
