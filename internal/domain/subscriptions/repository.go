package subscriptions

import (
	"context"
)

// ListQuery defines the query parameters for listing subscriptions.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	User                    string
	State                   *string
}

// Repository defines the interface for subscriptions persistence operations.
type Repository interface {
	// GetByIDForOrganization returns the subscription only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Subscription, error)
	List(ctx context.Context, q ListQuery) ([]*Subscription, int64, error)
	Create(ctx context.Context, sub *Subscription) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, sub *Subscription) error
	// Delete removes the subscription only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
