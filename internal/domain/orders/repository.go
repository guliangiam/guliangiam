package orders

import (
	"context"
)

// ListQuery defines the query parameters for listing orders.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	User                    string
	State                   *string
}

// Repository defines the interface for orders persistence operations.
type Repository interface {
	// GetByID is the system-scoped lookup. No HTTP management route may use
	// it: every id-bearing management route must use
	// GetByIDForOrganization so a caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Order, error)
	// GetByIDForOrganization returns the order only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Order, error)
	GetByTradeNo(ctx context.Context, tradeNo string) (*Order, error)
	List(ctx context.Context, q ListQuery) ([]*Order, int64, error)
	Create(ctx context.Context, order *Order) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, order *Order) error
	// Delete removes the order only when it belongs to organizationID; zero
	// affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
