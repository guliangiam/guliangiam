package products

import (
	"context"
)

// ListQuery defines the query parameters for listing products.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	State                   *string
	IsRecharge              *bool
}

// Repository defines the interface for products persistence operations.
type Repository interface {
	// GetByID is the system-scoped lookup. HTTP-facing flows, management routes
	// and product purchase must use GetByIDForOrganization so a caller-supplied
	// id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Product, error)
	// GetByIDForOrganization returns the product only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Product, error)
	List(ctx context.Context, q ListQuery) ([]*Product, int64, error)
	Create(ctx context.Context, product *Product) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, product *Product) error
	// Delete removes the product only when it belongs to organizationID; zero
	// affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
