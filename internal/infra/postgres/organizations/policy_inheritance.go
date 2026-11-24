package organizations

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const queryListPolicyInheritance = `
SELECT policy_key, mode, COALESCE(source_organization_id::text, '')
FROM guliangiam.organization_policy_inheritance
WHERE organization_id = $1
ORDER BY policy_key`

// ListPolicyInheritance returns the declared rules for one tenant organization.
func (r Repository) ListPolicyInheritance(ctx context.Context, organizationID string) ([]domain.PolicyInheritanceRule, error) {
	rows, err := r.db.Query(ctx, queryListPolicyInheritance, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list policy inheritance")
	}
	defer rows.Close()
	rules := make([]domain.PolicyInheritanceRule, 0)
	for rows.Next() {
		var rule domain.PolicyInheritanceRule
		var mode string
		if err := rows.Scan(&rule.PolicyKey, &mode, &rule.SourceOrganizationID); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan policy inheritance")
		}
		rule.Mode = domain.PolicyMode(mode)
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "iterate policy inheritance")
	}
	return rules, nil
}
