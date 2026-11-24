package webhooks

import "time"

type webhookRow struct {
	ID                    string
	OrganizationID        string
	Name                  string
	DisplayName           string
	URL                   string
	Method                string
	Headers               string
	Events                []byte
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
	TokenFields           []string
	ObjectFields          []string
	IsUserExtended        bool
	SingleOrgOnly         bool
	MaxRetries            int
	RetryInterval         int
	UseExponentialBackoff bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Revision              int64
}
