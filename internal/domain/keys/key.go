package keys

import "time"

type Key struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	KeyType        string     `json:"keyType"`
	PublicKey      string     `json:"publicKey"`
	PrivateKey     string     `json:"-"`
	Algorithm      string     `json:"algorithm"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	IsEnabled      bool       `json:"isEnabled"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	Revision       int64      `json:"revision"`
}
