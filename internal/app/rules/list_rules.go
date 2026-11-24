package rules

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
)

type ListRulesQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Scope          string
	RuleType       string
	IsEnabled      *bool
}

func (s Service) ListRules(ctx context.Context, query ListRulesQuery) ([]RuleListItem, int64, error) {
	rules, total, err := s.UOW.Rules().List(ctx, domain.ListQuery{
		Page:           query.Page,
		PageSize:       query.PageSize,
		OrganizationID: query.OrganizationID,
		Scope:          query.Scope,
		RuleType:       query.RuleType,
		IsEnabled:      query.IsEnabled,
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]RuleListItem, 0, len(rules))
	for _, rule := range rules {
		items = append(items, toListItem(rule))
	}

	return items, total, nil
}
