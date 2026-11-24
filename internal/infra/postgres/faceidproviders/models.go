package faceidproviders

import "time"

type faceIdProviderRow struct {
	ID              string
	OrganizationID  string
	Name            string
	ProviderType    string
	AccessKeyID     string
	AccessKeySecret string
	Endpoint        string
	Config          []byte
	IsEnabled       bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Revision        int64
}
