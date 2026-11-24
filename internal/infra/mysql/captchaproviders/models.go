package captchaproviders

import "time"

// captchaProviderRow 是 captchaproviders 表的行模型，
// JSON 列用 []byte 承接，mapper 层再反序列化。
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
