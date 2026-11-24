package forms

import (
	"context"
)

// ListQuery defines the query parameters for listing forms.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
	Type                    string
}

// Repository defines the interface for forms persistence operations.
type Repository interface {
	// GetByID returns the form only when it belongs to organizationID; a
	// foreign or missing row is reported as not found.
	GetByID(ctx context.Context, id, organizationID string) (*Form, error)
	List(ctx context.Context, q ListQuery) ([]*Form, int64, error)
	Create(ctx context.Context, form *Form) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, form *Form) error
	// Delete removes the form only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
