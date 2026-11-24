package radiusconfigs

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	IsEnabled      *bool
	AuthProtocol   string
}

type Repository interface {
	// GetByID returns the config only when it belongs to organizationID; a
	// foreign or missing row is reported as not found.
	GetByID(ctx context.Context, id, organizationID string) (*RadiusConfig, error)
	List(ctx context.Context, query ListQuery) ([]*RadiusConfig, int64, error)
	Create(ctx context.Context, config *RadiusConfig) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, config *RadiusConfig) error
	// Delete removes the config only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
