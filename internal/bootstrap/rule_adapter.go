package bootstrap

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/domain/rules"
	"github.com/guliangiam/guliangiam/internal/transport/http/middleware"
)

// ruleQueryAdapter adapts the domain rules.Repository to the
// middleware.RuleQueryService interface, bridging the domain and transport
// layers through the bootstrap composition root.
type ruleQueryAdapter struct {
	repo rules.Repository
}

// newRuleQueryService creates a middleware.RuleQueryService backed by the
// domain rules.Repository. This adapter lives in the bootstrap layer because
// bootstrap is allowed to import all other layers.
func newRuleQueryService(repo rules.Repository) middleware.RuleQueryService {
	return &ruleQueryAdapter{repo: repo}
}

func (a *ruleQueryAdapter) ListEnabledByType(ctx context.Context, ruleType string) ([]middleware.RuleEntry, error) {
	enabled := true
	rs, _, err := a.repo.List(ctx, rules.ListQuery{
		RuleType:  ruleType,
		IsEnabled: &enabled,
	})
	if err != nil {
		return nil, err
	}

	entries := make([]middleware.RuleEntry, len(rs))
	for i, r := range rs {
		entries[i] = middleware.RuleEntry{
			Name:       r.Name,
			Action:     r.Action,
			IsEnabled:  r.IsEnabled,
			Conditions: r.Conditions,
		}
	}
	return entries, nil
}
