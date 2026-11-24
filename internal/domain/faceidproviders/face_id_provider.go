package faceidproviders

import "time"

type FaceIdProvider struct {
	ID              string
	OrganizationID  string
	Name            string
	ProviderType    string // Aliyun
	AccessKeyID     string
	AccessKeySecret string
	Endpoint        string
	Config          map[string]any
	IsEnabled       bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Revision        int64
	// SecretNeedsRotation reports that AccessKeySecret was read from a legacy
	// plaintext row (A-6.1 backfill not applied yet). Repositories set it on
	// read; it is never persisted and never carries the secret value.
	SecretNeedsRotation bool
}
