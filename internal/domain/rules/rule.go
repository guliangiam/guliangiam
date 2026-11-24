package rules

import "time"

type Rule struct {
	ID             string
	OrganizationID string
	Name           string
	Scope          string // org, application
	RuleType       string // IP, UA, Rate, WAF
	Conditions     []map[string]any
	Action         string // Allow, Block, Captcha
	Priority       int
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
