package entries

import (
	"context"
)

// ListQuery defines the query parameters for listing entries.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	Type                    string
	Application             string
}

// Repository defines the interface for entries persistence operations.
type Repository interface {
	// GetByID is the system-scoped lookup kept for internal callers.
	// Request by-ID reads, including the transcript read, must use
	// GetByIDForOrganization so a caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Entry, error)
	// GetByIDForOrganization returns the entry only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Entry, error)
	List(ctx context.Context, q ListQuery) ([]*Entry, int64, error)
	Create(ctx context.Context, entry *Entry) error
	// Update uses the entity OrganizationID as the tenant predicate.
	Update(ctx context.Context, entry *Entry) error
	// Delete removes the entry only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
