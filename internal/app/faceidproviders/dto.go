package faceidproviders

import "time"

type FaceIdProviderListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	ProviderType   string    `json:"providerType"`
	AccessKeyID    string    `json:"accessKeyId"`
	Endpoint       string    `json:"endpoint"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	// AccessKeySecretNeedsRotation marks a row whose stored secret is still
	// legacy plaintext; run `migrate encrypt-faceid-secrets`. The secret itself
	// is never returned (detail uses AccessKeySecretSet/Masked).
	AccessKeySecretNeedsRotation bool `json:"accessKeySecretNeedsRotation"`
}

type FaceIdProviderDetail struct {
	FaceIdProviderListItem
	AccessKeySecretSet    bool           `json:"accessKeySecretSet"`
	AccessKeySecretMasked string         `json:"accessKeySecretMasked,omitempty"`
	Config                map[string]any `json:"config"`
	Revision              int64          `json:"revision"`
}
