package platform_admins

import "time"

// PlatformAdminDetail 是平台管理员的读侧 DTO（BE-A-05）。
// transport 层不得 import domain（Clean Architecture Rule 2），因此管理面
// API 的响应投影统一落在 app 层，字段与 domain 实体一一对应，json tag camelCase。
type PlatformAdminDetail struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	CreatedBy string    `json:"createdBy"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Revision  int64     `json:"revision"`
}

// ScopeDetail 是平台管理员租户作用域的读侧 DTO。
// IsBootstrap 只读披露：种子/运维写入的免审批标记，应用层不可创建。
type ScopeDetail struct {
	ID              string     `json:"id"`
	PlatformAdminID string     `json:"platformAdminId"`
	ScopeType       string     `json:"scopeType"`
	TargetTenants   []string   `json:"targetTenants"`
	ApprovedBy      *string    `json:"approvedBy"`
	ApprovedAt      *time.Time `json:"approvedAt"`
	IsBootstrap     bool       `json:"isBootstrap"`
	ExpiresAt       time.Time  `json:"expiresAt"`
	CreatedAt       time.Time  `json:"createdAt"`
}
