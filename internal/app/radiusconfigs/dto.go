package radiusconfigs

import "time"

type RadiusConfigListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	AuthProtocol   string    `json:"authProtocol"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type RadiusConfigDetail struct {
	RadiusConfigListItem
	SharedSecretSet    bool   `json:"sharedSecretSet"`
	SharedSecretMasked string `json:"sharedSecretMasked,omitempty"`
	Revision           int64  `json:"revision"`
}
