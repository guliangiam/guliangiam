package radiusaccountings

import (
	"context"
)

// ListQuery defines the query parameters for listing radiusaccountings.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	Username                string
}

// Repository defines the interface for radiusaccountings persistence operations.
type Repository interface {
	// GetByID returns the accounting only when it belongs to organizationID;
	// a foreign or missing row is reported as not found.
	GetByID(ctx context.Context, id, organizationID string) (*RadiusAccounting, error)
	List(ctx context.Context, q ListQuery) ([]*RadiusAccounting, int64, error)
	Create(ctx context.Context, ra *RadiusAccounting) error
	// Update scopes the written row by the entity's own OrganizationID, not by
	// a caller-supplied value.
	Update(ctx context.Context, ra *RadiusAccounting) error
	// Delete removes the accounting only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
