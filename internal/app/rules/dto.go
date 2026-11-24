package rules

import "time"

type RuleListItem struct {
	ID             string           `json:"id"`
	OrganizationID string           `json:"organizationId"`
	Name           string           `json:"name"`
	Scope          string           `json:"scope"`
	RuleType       string           `json:"ruleType"`
	Conditions     []map[string]any `json:"conditions"`
	Action         string           `json:"action"`
	Priority       int              `json:"priority"`
	IsEnabled      bool             `json:"isEnabled"`
	CreatedAt      time.Time        `json:"createdAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
}

type RuleDetail struct {
	RuleListItem
	Revision int64 `json:"revision"`
}
