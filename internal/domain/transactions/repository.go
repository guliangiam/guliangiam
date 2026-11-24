package transactions

import (
	"context"
)

// ListQuery defines the query parameters for listing transactions.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	UserID                  string
	OrderID                 string
	Status                  *string
	TransactionType         *string
}

// Repository defines the interface for transactions persistence operations.
type Repository interface {
	// GetByID is the system-scoped lookup used by refunds and user-scoped
	// sub-resources. Management routes must use GetByIDForOrganization so a
	// caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Transaction, error)
	// GetByIDForOrganization returns the transaction only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Transaction, error)
	List(ctx context.Context, q ListQuery) ([]*Transaction, int64, error)
	Create(ctx context.Context, tx *Transaction) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, tx *Transaction) error
	// Delete removes the transaction only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
