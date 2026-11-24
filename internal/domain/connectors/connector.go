// Package connectors 定义出站 Provisioning 连接器的域模型。
//
// 持久化接口由 infra/postgres/connectors 实现（MySQL 运行期受 OPEN-API-37
// 约束，本批不提供镜像仓储）；测试用 outbound/scim.StubConnectorRepo。
//
// Outbox 路由键：所有 SCIM 出站事件的 aggregate_type = AggregateType。
package connectors

import (
	"context"
	"time"
)

// AggregateType 是 Outbox 事件路由键。ADR-030 规定 SCIM 出站事件走
// aggregate_type="scim_outbound"，Worker 通过 Registry.Get 找到 DeliveryHandler。
const AggregateType = "scim_outbound"

// ──────────────────────────────────────────────────────────────────────
// SCIM Outbound 事件类型（ADR-030 §8.5 事件清单）
// 这些值直接写进 OutboxItem.EventType，由 DeliveryHandler 按 eventType 映射 HTTP 方法。
// ──────────────────────────────────────────────────────────────────────

const (
	// User Provisioning
	EvtUserCreated     = "user.created"     // POST /Users
	EvtUserUpdated     = "user.updated"     // PUT /Users/{id}
	EvtUserDeactivated = "user.deactivated" // PATCH /Users/{id} (active=false)

	// Group Provisioning
	EvtGroupCreated       = "group.created"
	EvtGroupUpdated       = "group.updated"
	EvtGroupDeleted       = "group.deleted"
	EvtGroupMemberAdded   = "group.member.added"
	EvtGroupMemberRemoved = "group.member.removed"

	// Role Provisioning (ADR-030 角色成员也下发，SCIM 中角色常作为 Group 下发)
	EvtRoleMemberAdded   = "role.member.added"
	EvtRoleMemberRemoved = "role.member.removed"
)

// ValidEventTypes 是 Worker 注册时的事件类型白名单校验（可选）。
var ValidEventTypes = []string{
	EvtUserCreated, EvtUserUpdated, EvtUserDeactivated,
	EvtGroupCreated, EvtGroupUpdated, EvtGroupDeleted,
	EvtGroupMemberAdded, EvtGroupMemberRemoved,
	EvtRoleMemberAdded, EvtRoleMemberRemoved,
}

// IsValidEventType 白名单校验辅助。
func IsValidEventType(et string) bool {
	for _, v := range ValidEventTypes {
		if v == et {
			return true
		}
	}
	return false
}

// Connector 是出站 Provisioning 连接器的最小域模型。
type Connector struct {
	ID             string
	OrganizationID string
	Name           string
	SCIMBaseURL    string // e.g. "https://example.com/scim/v2"
	BearerToken    string // 明文仅在内存内流动；持久化时必须走 FieldEncryptor 信封加密
	IsEnabled      bool
	TimeoutMs      int // 默认 5000
	MaxRatePerMin  int // 下游速率上限；0 = 不限；默认 300
	Revision       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ConnectorRepository 是租户绑定的连接器配置 CRUD 端口。
//
// 每个方法都显式接收 organizationID：调用方必须传入可信租户（HTTP 侧由
// authtransport.TrustedOrganizationID 派生），仓储把它作为 SQL 谓词，
// 不信任实体字段或请求参数。不存在跨租户查询方法。
//
// Update 使用乐观锁：connector.Revision 是调用方读到的当前版本；仓储在
// revision 不匹配时返回 Conflict，且不写入任何列。成功时仓储把
// connector.Revision 更新为自增后的值。
type ConnectorRepository interface {
	Get(ctx context.Context, connectorID, organizationID string) (*Connector, error)
	List(ctx context.Context, organizationID string) ([]*Connector, error)
	Create(ctx context.Context, organizationID string, connector *Connector) error
	Update(ctx context.Context, organizationID string, connector *Connector) error
	Delete(ctx context.Context, organizationID, connectorID string) error
}

// DefaultTimeoutMs 是 Connector.TimeoutMs 的默认值。
const DefaultTimeoutMs = 5000

// DefaultMaxRatePerMin 是 Connector.MaxRatePerMin 的默认值。
const DefaultMaxRatePerMin = 300
