package pricings

import (
	"context"
)

// ListQuery defines the query parameters for listing pricings.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
}

// Repository defines the interface for pricings persistence operations.
type Repository interface {
	// GetByID returns the pricing only when it belongs to organizationID; a
	// foreign or missing row is reported as not found.
	GetByID(ctx context.Context, id, organizationID string) (*Pricing, error)
	List(ctx context.Context, q ListQuery) ([]*Pricing, int64, error)
	Create(ctx context.Context, pricing *Pricing) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, pricing *Pricing) error
	// Delete removes the pricing only when it belongs to organizationID; zero
	// affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
