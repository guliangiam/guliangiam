package webhooks

import "time"

type Webhook struct {
	ID                    string
	OrganizationID        string
	Name                  string
	DisplayName           string
	URL                   string
	Method                string   // POST, GET, PUT
	Headers               string   // JSON headers
	Events                []string // event types to subscribe
	IsEnabled             bool
	Secret                string
	RetryCount            int
	TimeoutMs             int
	Organization          string
	ContentType           string
	CustomBody            string
	CustomHeaders         string // JSON string (like Headers)
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
