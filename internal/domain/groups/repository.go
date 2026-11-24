package groups

import "context"

type Repository interface {
	// GetByID is the system-scoped lookup used where the caller has no request
	// tenant yet (id-collision checks) or resolves ownership itself (social
	// signup, LDAP/syncer jobs). Management and SCIM by-ID routes must use
	// GetByIDForOrganization.
	GetByID(ctx context.Context, id string) (*Group, error)
	// GetByIDForOrganization returns the group only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Group, error)
	List(ctx context.Context, query ListQuery) ([]*Group, int64, error)
	Create(ctx context.Context, group *Group) error
	// Update 以实体 OrganizationID 作为租户谓词。
	Update(ctx context.Context, group *Group) error
	// Delete 按 id + organizationID 删除；0 行返回 NotFound。
	Delete(ctx context.Context, id, organizationID string) error
	CountChildrenByParentID(ctx context.Context, organizationID string, parentID string) (int64, error)
}
