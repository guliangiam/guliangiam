package adapters

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
	DatabaseType   string
}

type Repository interface {
	// GetByID 按 id + organizationID 读取；跨租户与不存在同样返回 NotFound。
	GetByID(ctx context.Context, id, organizationID string) (*Adapter, error)
	List(ctx context.Context, query ListQuery) ([]*Adapter, int64, error)
	Create(ctx context.Context, adapter *Adapter) error
	// Update 以实体 OrganizationID 作为租户谓词。
	Update(ctx context.Context, adapter *Adapter) error
	// Delete 按 id + organizationID 删除；0 行返回 NotFound。
	Delete(ctx context.Context, id, organizationID string) error
}
