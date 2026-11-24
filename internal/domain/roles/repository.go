package roles

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type Repository interface {
	// GetByIDForOrganization returns the role only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Role, error)
	List(ctx context.Context, query ListQuery) ([]*Role, int64, error)
	Create(ctx context.Context, role *Role) error
	// Update 以实体 OrganizationID 作为租户谓词。
	Update(ctx context.Context, role *Role) error
	// Delete 按 id + organizationID 删除；0 行返回 NotFound。
	Delete(ctx context.Context, id, organizationID string) error
}
