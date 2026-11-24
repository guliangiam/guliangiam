package platformadmins

import "context"

// PlatformAdminRepository 平台管理员 CRUD。
// 所有操作走 TenantFilterSystem ctx（RLS: is_super_admin() 才能读写）。
type PlatformAdminRepository interface {
	GetByID(ctx context.Context, id string) (*PlatformAdmin, error)
	GetByUserID(ctx context.Context, userID string) (*PlatformAdmin, error)
	List(ctx context.Context, opts ListOptions) ([]*PlatformAdmin, int64, error)
	Create(ctx context.Context, admin *PlatformAdmin) error
	Update(ctx context.Context, admin *PlatformAdmin) error
	SetActive(ctx context.Context, id string, active bool) error
	Delete(ctx context.Context, id string) error
}

// PlatformAdminScopeRepository 作用域 CRUD + 查询。
// 所有操作走 TenantFilterSystem ctx（RLS: is_super_admin() 才能读写）。
type PlatformAdminScopeRepository interface {
	GetByID(ctx context.Context, id string) (*PlatformAdminScope, error)
	ListByPlatformAdmin(ctx context.Context, adminID string, opts ScopeListOptions) ([]*PlatformAdminScope, error)
	Create(ctx context.Context, scope *PlatformAdminScope) error
	Approve(ctx context.Context, scopeID string, approverID string) error
	Delete(ctx context.Context, id string) error

	// HasActiveScopeForTenant 查询指定 admin 是否有覆盖目标租户的有效 scope。
	// 条件：expires_at > now() AND (scope_type='all' OR target_tenants 包含 tenantID)。
	// 用于 ScopeAuthorizer Gate 7（hot path）。
	HasActiveScopeForTenant(ctx context.Context, adminID string, tenantID string) (bool, error)
}

// ListOptions 列表查询选项
type ListOptions struct {
	Page     int
	PageSize int
	Active   *bool  // nil=不过滤；true=只要 is_active；false=只要停用
	Keyword  string // 搜索 user_id（精确或模糊）
}

// ScopeListOptions 作用域列表选项
type ScopeListOptions struct {
	OnlyActive bool // true=只要 expires_at > now() 的
}
