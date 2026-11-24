package rules

import domain "github.com/guliangiam/guliangiam/internal/domain/rules"

func toListItem(rule *domain.Rule) RuleListItem {
	conditions := make([]map[string]any, 0, len(rule.Conditions))
	conditions = append(conditions, rule.Conditions...)
	return RuleListItem{
		ID:             rule.ID,
		OrganizationID: rule.OrganizationID,
		Name:           rule.Name,
		Scope:          rule.Scope,
		RuleType:       rule.RuleType,
		Conditions:     conditions,
		Action:         rule.Action,
		Priority:       rule.Priority,
		IsEnabled:      rule.IsEnabled,
		CreatedAt:      rule.CreatedAt,
		UpdatedAt:      rule.UpdatedAt,
	}
}

func toDetail(rule *domain.Rule) RuleDetail {
	return RuleDetail{
		RuleListItem: toListItem(rule),
		Revision:     rule.Revision,
	}
}
