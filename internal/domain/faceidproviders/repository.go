package faceidproviders

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	IsEnabled      *bool
}

type Repository interface {
	// GetByID returns the provider only when it belongs to organizationID; a
	// foreign or missing row is reported as not found.
	GetByID(ctx context.Context, id, organizationID string) (*FaceIdProvider, error)
	List(ctx context.Context, query ListQuery) ([]*FaceIdProvider, int64, error)
	Create(ctx context.Context, provider *FaceIdProvider) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, provider *FaceIdProvider) error
	// Delete removes the provider only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
