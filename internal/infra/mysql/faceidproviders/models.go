package faceidproviders

import "time"

// faceIdProviderRow 是 faceidproviders 表的行模型。
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
