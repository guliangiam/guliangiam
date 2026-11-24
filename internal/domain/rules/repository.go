package rules

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Scope          string
	RuleType       string
	IsEnabled      *bool
}

type Repository interface {
	// GetByID returns the rule only when it belongs to organizationID; a
	// foreign or missing row is reported as not found.
	GetByID(ctx context.Context, id, organizationID string) (*Rule, error)
	List(ctx context.Context, query ListQuery) ([]*Rule, int64, error)
	Create(ctx context.Context, rule *Rule) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, rule *Rule) error
	// Delete removes the rule only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
