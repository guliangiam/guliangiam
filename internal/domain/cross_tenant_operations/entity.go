package cross_tenant_operations

import (
	"time"

	"github.com/google/uuid"
)

// Ticket 跨租户操作票实体。
//
// 存在于 guliangiam.cross_tenant_operations 表（V5 migration）。
//
// 状态机：
//
//	pending → approved → used
//	        → rejected
//	approved → expired (expires_at > now())
//
// actor_id 语义（按 V5 schema 注释）：存 users 表的 id，不是 platform_admins.id。
// 但有一个关联 FK approved_by → platform_admins(id)，所以审批者必须是 registered admin。
type Ticket struct {
	ID           string     `json:"id"`
	ScopeID      *string    `json:"scopeId"`      // 关联的 scope，nullable
	ActorID      string     `json:"actorId"`      // 操作人 users.id（不是 platform_admins.id）
	SourceTenant *string    `json:"sourceTenant"` // nullable
	TargetTenant string     `json:"targetTenant"`
	ResourceType string     `json:"resourceType"`
	Action       string     `json:"action"`
	RiskLevel    string     `json:"riskLevel"`
	Status       string     `json:"status"`     // pending/approved/rejected/used/expired
	ApprovedBy   *string    `json:"approvedBy"` // 审批者 platform_admins.id，nullable
	ApprovedAt   *time.Time `json:"approvedAt"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	UsedAt       *time.Time `json:"usedAt"`
	TraceID      *string    `json:"traceId"`
	Reason       *string    `json:"reason"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// ValidStatuses ticket 状态机合法值。
var ValidStatuses = map[string]bool{
	StatusPending:  true,
	StatusApproved: true,
	StatusRejected: true,
	StatusUsed:     true,
	StatusExpired:  true,
}

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
	StatusUsed     = "used"
	StatusExpired  = "expired"
)

// IsApprovedAndActive ticket 是否处于有效状态（approved + 未过期）。
func (t *Ticket) IsApprovedAndActive() bool {
	if t.Status != StatusApproved {
		return false
	}
	if t.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

// IsTerminal 是否终态（used/rejected/expired — 不可再变更）。
func (t *Ticket) IsTerminal() bool {
	switch t.Status {
	case StatusUsed, StatusRejected, StatusExpired:
		return true
	}
	return false
}

// TargetTenantUUID 返回 TargetTenant 作为 uuid.UUID（辅助方法）。
func (t *Ticket) TargetTenantUUID() (uuid.UUID, error) {
	return uuid.Parse(t.TargetTenant)
}
