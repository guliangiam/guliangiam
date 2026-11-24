package rules

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func toRule(row ruleRow) (*domain.Rule, error) {
	rule := &domain.Rule{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		Scope:          row.Scope,
		RuleType:       row.RuleType,
		Action:         row.Action,
		Priority:       row.Priority,
		IsEnabled:      row.IsEnabled,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	if err := shared.UnmarshalJSON(row.Conditions, &rule.Conditions); err != nil {
		return nil, err
	}
	if rule.Conditions == nil {
		rule.Conditions = []map[string]any{}
	}
	return rule, nil
}
