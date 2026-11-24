package keys

import (
	"context"
)

// ListQuery defines the query parameters for listing keys.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	KeyType                 *string
	IsEnabled               *bool
}

// Repository defines the interface for keys persistence operations.
type Repository interface {
	GetByID(ctx context.Context, id, organizationID string) (*Key, error)
	List(ctx context.Context, q ListQuery) ([]*Key, int64, error)
	Create(ctx context.Context, key *Key) error
	Update(ctx context.Context, key *Key) error
	Delete(ctx context.Context, id, organizationID string) error
}
