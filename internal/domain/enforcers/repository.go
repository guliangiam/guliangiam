package enforcers

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type Policy struct {
	PType string
	V0    string
	V1    string
	V2    string
	V3    string
	V4    string
	V5    string
}

type Repository interface {
	// GetByID is the system-scoped lookup used by permission management (which
	// resolves the enforcer through an owning permission row) and by the policy
	// action routes outside the management by-ID scope. Management routes must
	// use GetByIDForOrganization so a caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Enforcer, error)
	// GetByIDForOrganization returns the enforcer only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Enforcer, error)
	List(ctx context.Context, query ListQuery) ([]*Enforcer, int64, error)
	Create(ctx context.Context, enforcer *Enforcer) error
	// Update uses the entity OrganizationID as the tenant predicate. The app
	// layer performs a scoped read first, so zero affected rows still means a
	// revision conflict.
	Update(ctx context.Context, enforcer *Enforcer) error
	// Delete removes the enforcer only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
