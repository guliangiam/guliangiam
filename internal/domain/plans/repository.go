package plans

import (
	"context"
)

// ListQuery defines the query parameters for listing plans.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	// State filters by the registry lifecycle state (§1.5). The former
	// `IsEnabled` boolean filter is gone with the column: filtering by
	// `active` is now the only way to ask "which plans are sellable".
	State       string
	Period      string
	IsExclusive *bool
}

// Repository defines the interface for plans persistence operations.
type Repository interface {
	// GetByID returns the plan only when it belongs to organizationID; a
	// foreign or missing row is reported as not found.
	GetByID(ctx context.Context, id, organizationID string) (*Plan, error)
	List(ctx context.Context, q ListQuery) ([]*Plan, int64, error)
	Create(ctx context.Context, plan *Plan) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, plan *Plan) error
	// Delete removes the plan only when it belongs to organizationID; zero
	// affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
