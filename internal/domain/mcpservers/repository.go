package mcpservers

import (
	"context"
)

// ListQuery defines the query parameters for listing mcpservers.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
}

// Repository defines the interface for mcpservers persistence operations.
type Repository interface {
	// GetByID is the system-scoped lookup retained for background tool
	// synchronization workers. Management by-ID routes (sync-tools/proxy) must
	// use GetByIDForOrganization or a scoped List so a caller-supplied id or
	// slug cannot cross tenants.
	GetByID(ctx context.Context, id string) (*McpServer, error)
	// GetByIDForOrganization returns the server only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*McpServer, error)
	List(ctx context.Context, q ListQuery) ([]*McpServer, int64, error)
	Create(ctx context.Context, server *McpServer) error
	// Update predicates on the entity OrganizationID. The app layer performs a
	// scoped read first, so zero affected rows means the row is gone (or was
	// never in this tenant) and is reported as not found.
	Update(ctx context.Context, server *McpServer) error
	// Delete removes the server only when it belongs to organizationID; zero
	// affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
