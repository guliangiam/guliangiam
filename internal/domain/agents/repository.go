package agents

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type Repository interface {
	GetByID(ctx context.Context, id, organizationID string) (*Agent, error)
	List(ctx context.Context, query ListQuery) ([]*Agent, int64, error)
	Create(ctx context.Context, agent *Agent) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, agent *Agent) error
	Delete(ctx context.Context, id, organizationID string) error
}
