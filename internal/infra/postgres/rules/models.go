package rules

import "time"

type ruleRow struct {
	ID             string
	OrganizationID string
	Name           string
	Scope          string
	RuleType       string
	Conditions     []byte
	Action         string
	Priority       int
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
