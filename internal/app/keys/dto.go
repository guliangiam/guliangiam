package keys

import (
	"time"
)

type ListKeysQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	KeyType                 *string
	IsEnabled               *bool
}

type CreateKeyCommand struct {
	OrganizationID string
	Name           string
	DisplayName    string
	KeyType        string
	PublicKey      string
	PrivateKey     string
	Algorithm      string
	ExpiresAt      *time.Time
	IsEnabled      bool
}

type PatchKeyCommand struct {
	OrganizationID string
	ID             string
	DisplayName    *string
	PublicKey      *string
	PrivateKey     *string
	Algorithm      *string
	ExpiresAt      *time.Time
	IsEnabled      *bool
	Revision       int64
}

type KeyListItem struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	KeyType        string     `json:"keyType"`
	Algorithm      string     `json:"algorithm"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	IsEnabled      bool       `json:"isEnabled"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type KeyDetail struct {
	KeyListItem
	PublicKey string `json:"publicKey"`
	Revision  int64  `json:"revision"`
}
