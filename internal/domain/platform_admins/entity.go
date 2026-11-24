package platformadmins

import (
	"time"

	"github.com/google/uuid"
)

// PlatformAdmin 平台管理员实体。
// 存在于 guliangiam.platform_admins 表（V5 migration）。
//
// 与 users.is_admin 的区别：
//
//	users.is_admin 是组织内的管理员（可管自己 org）。
//	platform_admins 是**平台级**管理员（可跨 org，受 scope 约束）。
//
// 激活/停用由 is_active 控制；scope 授予跨租户操作权限。
type PlatformAdmin struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	CreatedBy string    `json:"createdBy"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Revision  int64     `json:"revision"`
}

// PlatformAdminScope 平台管理员的租户作用域。
// 存在于 guliangiam.platform_admin_scopes 表（V5 migration）。
//
// scope_type 含义：
//
//	'all'      → 平台管理员可操作**所有**租户（需审批；种子内置管理员由 is_bootstrap 豁免）
//	'targeted' → 平台管理员只能操作 target_tenants[] 列出的租户
//
// 有效期：最长 1 年（DB CHECK 约束）。
// 有效性唯一口径（未过期 && (is_bootstrap || approved_by 非空)）定义在
// internal/infra/postgres/platform_admins/sql.go 的 scopeValidity，本实体不再提供
// 第二套判定方法。
type PlatformAdminScope struct {
	ID              string     `json:"id"`
	PlatformAdminID string     `json:"platformAdminId"`
	ScopeType       string     `json:"scopeType"`     // 'all' | 'targeted'
	TargetTenants   []string   `json:"targetTenants"` // scope_type='targeted' 时有效
	ApprovedBy      *string    `json:"approvedBy"`    // 审批者的 users.id（FK → users.id）
	ApprovedAt      *time.Time `json:"approvedAt"`
	IsBootstrap     bool       `json:"isBootstrap"` // 种子/运维写入的免审批标记（应用层不可创建）
	ExpiresAt       time.Time  `json:"expiresAt"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// ValidScopeTypes — scope_type 允许值
var ValidScopeTypes = map[string]bool{
	ScopeTypeAll:      true,
	ScopeTypeTargeted: true,
}

const (
	ScopeTypeAll      = "all"
	ScopeTypeTargeted = "targeted"
)

// Covers 判断 scope 是否覆盖指定 tenant。
func (s *PlatformAdminScope) Covers(tenantID uuid.UUID) bool {
	if s.ScopeType == ScopeTypeAll {
		return true
	}
	targetStr := tenantID.String()
	for _, t := range s.TargetTenants {
		if t == targetStr {
			return true
		}
	}
	return false
}
