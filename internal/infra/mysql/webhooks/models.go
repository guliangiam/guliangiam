package webhooks

import "time"

// webhookRow 是 MySQL webhooks 表的扫描目标。
// 所有列均 NOT NULL（对照 PG 版 webhookRow，无 *string/*time.Time 指针列），
// 因此不需要 sql.Null* 类型。
type webhookRow struct {
	ID                    string
	OrganizationID        string
	Name                  string
	DisplayName           string
	URL                   string
	Method                string
	Headers               string
	Events                []byte // JSON
	IsEnabled             bool
	Secret                string
	RetryCount            int
	TimeoutMs             int
	Organization          string
	ContentType           string
	CustomBody            string
	CustomHeaders         string
	CertID                string
	IsError               bool
	ErrorText             string
	TokenFields           []byte // JSON
	ObjectFields          []byte // JSON
	IsUserExtended        bool
	SingleOrgOnly         bool
	MaxRetries            int
	RetryInterval         int
	UseExponentialBackoff bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Revision              int64
}
