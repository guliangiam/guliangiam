package permissions

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
	State          string
	ResourceType   string
	Effect         string
}

type Repository interface {
	// GetByID is the unscoped, system-scoped lookup reserved for infrastructure
	// diagnostics and repair tooling. Every request-scoped by-ID path must use
	// GetByIDForOrganization so a caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Permission, error)
	// GetByIDForOrganization returns the permission only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Permission, error)
	List(ctx context.Context, query ListQuery) ([]*Permission, int64, error)
	Create(ctx context.Context, permission *Permission) error
	// Update uses the entity OrganizationID as the tenant predicate. The app
	// layer performs a scoped read first, so zero affected rows still means a
	// revision conflict.
	Update(ctx context.Context, permission *Permission) error
	// Delete removes the permission only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}

type CasbinPolicyManager interface {
	Enforce(ctx context.Context, enforcerID, modelText string, rvals ...interface{}) (bool, error)
	BatchEnforce(ctx context.Context, enforcerID, modelText string, rvals [][]interface{}) ([]bool, error)
	AddPolicy(enforcerID, modelText string, params []string) error
	RemovePolicy(enforcerID, modelText string, params []string) error
	GetPolicies(enforcerID, modelText string) ([][]string, error)
	GetFilteredPolicies(enforcerID, modelText string, fieldIndex int, fieldValues ...string) ([][]string, error)
	UpdatePolicy(enforcerID, modelText string, oldPolicy []string, newPolicy []string) error
	RemoveFilteredPolicy(enforcerID, modelText string, fieldIndex int, fieldValues ...string) error
	ReloadPolicy(enforcerID, modelText string) error
	GetAllSubjects(enforcerID, modelText string) ([]string, error)
	GetAllObjects(enforcerID, modelText string) ([]string, error)
	GetAllActions(enforcerID, modelText string) ([]string, error)
	GetAllRoles(enforcerID, modelText string) ([]string, error)
	AddRoleForUser(enforcerID, modelText string, user, role string) error
	GetOrgEnforcerPolicies(orgID string) ([][]string, error)
	AddOrgPolicy(orgID string, params []string) error
	RemoveOrgFilteredPolicy(orgID string, fieldIndex int, fieldValues ...string) error
	AddOrgRoleForUser(orgID string, user, role string) error
	RemoveOrgFilteredGroupingPolicy(orgID string, fieldIndex int, fieldValues ...string) error
	InvalidateAll()
	// GetOrgSubjectPolicies returns every p-policy row applicable to the given
	// subject after expanding role inheritance (g-policies).
	// Each row is [sub, dom, obj, act].
	GetOrgSubjectPolicies(orgID string, subject string) ([][]string, error)
}
