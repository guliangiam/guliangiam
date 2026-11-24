package syncers

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type Repository interface {
	// GetByIDForOrganization returns the syncer only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	// Every by-ID read is tenant-scoped, including run/test-connection, so
	// there is no system-scoped lookup that could bypass the tenant.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Syncer, error)
	List(ctx context.Context, query ListQuery) ([]*Syncer, int64, error)
	Create(ctx context.Context, syncer *Syncer) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, syncer *Syncer) error
	// Delete removes the syncer only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
