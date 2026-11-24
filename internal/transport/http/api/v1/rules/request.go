package rules

import apprules "github.com/guliangiam/guliangiam/internal/app/rules"

type ListRulesRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Scope          string
	RuleType       string
	IsEnabled      *bool
}

type CreateRuleRequest struct {
	OrganizationID string           `json:"organizationId"`
	Name           string           `json:"name"`
	Scope          string           `json:"scope"`
	RuleType       string           `json:"ruleType"`
	Conditions     []map[string]any `json:"conditions,omitempty"`
	Action         string           `json:"action"`
	Priority       int              `json:"priority,omitempty"`
	IsEnabled      *bool            `json:"isEnabled,omitempty"`
}

type PatchRuleRequest struct {
	Name       *string          `json:"name,omitempty"`
	Scope      *string          `json:"scope,omitempty"`
	RuleType   *string          `json:"ruleType,omitempty"`
	Conditions []map[string]any `json:"conditions,omitempty"`
	Action     *string          `json:"action,omitempty"`
	Priority   *int             `json:"priority,omitempty"`
	IsEnabled  *bool            `json:"isEnabled,omitempty"`
	Revision   int64            `json:"revision,omitempty"`
}

type ValidateRuleRequest struct {
	RuleType   string   `json:"ruleType"`
	Conditions []string `json:"conditions"`
}

func (r CreateRuleRequest) toCommand() apprules.CreateRuleCommand {
	return apprules.CreateRuleCommand{
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		Scope:          r.Scope,
		RuleType:       r.RuleType,
		Conditions:     r.Conditions,
		Action:         r.Action,
		Priority:       r.Priority,
		IsEnabled:      r.IsEnabled,
	}
}

func (r PatchRuleRequest) toCommand(id string) apprules.PatchRuleCommand {
	command := apprules.PatchRuleCommand{
		ID:        id,
		Name:      r.Name,
		Scope:     r.Scope,
		RuleType:  r.RuleType,
		Action:    r.Action,
		Priority:  r.Priority,
		IsEnabled: r.IsEnabled,
		Revision:  r.Revision,
	}
	if r.Conditions != nil {
		command.Conditions = r.Conditions
	}
	return command
}
