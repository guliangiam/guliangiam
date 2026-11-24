package organizations

import "context"

// PolicyInheritanceRepository reads the declared inheritance rules for one
// organization. Rules are tenant-scoped by the caller's transaction/session.
type PolicyInheritanceRepository interface {
	ListPolicyInheritance(ctx context.Context, organizationID string) ([]PolicyInheritanceRule, error)
}
