package cross_tenant_operations

import "context"

// TicketRepository 跨租户操作票 CRUD + 查询。
// 所有操作走 TenantFilterSystem ctx（RLS: is_super_admin() 才能读写）。
//
// 关键语义：actor_id 存 users 表的 id（不是 platform_admins.id）。
//
//	参见 V5 schema 注释 + AllowIfScoped latent bug fix。
type TicketRepository interface {
	// ── Hot path 查询（AllowIfScoped Gate 2/8） ────────────────────

	// HasApprovedTicket 查询指定 actor 是否有覆盖条件的有效 ticket。
	//
	// 参数说明：
	//   - actorUserID:   users 表的 id（V5 schema actor_id 语义）
	//   - targetTenant:  可以空字符串 — 空时不过滤 target_tenant
	//   - resourceType:  可以空字符串 — 空时不过滤 resource_type
	//   - action:        可以空字符串 — 空时不过滤 action
	//
	// 条件：status = 'approved' AND expires_at > now()
	// 这是 AllowIfScoped ScopeAuthorizer 的核心热路径。
	HasApprovedTicket(ctx context.Context, actorUserID string, targetTenant string, resourceType string, action string) (bool, error)

	// ── CRUD（Service 层 ticket 生命周期管理） ──────────────────────

	GetByID(ctx context.Context, id string) (*Ticket, error)
	Create(ctx context.Context, ticket *Ticket) error

	// Approve 将 ticket 从 pending → approved（state machine transition）。
	// approvedBy 必须是 platform_admins.id（FK 约束）。
	Approve(ctx context.Context, ticketID string, approverPlatformAdminID string) error

	// Reject 将 ticket 从 pending → rejected。
	Reject(ctx context.Context, ticketID string, reason string) error

	// MarkUsed 将 ticket 从 approved → used（操作执行后标记）。
	MarkUsed(ctx context.Context, ticketID string) error

	// ListByActor 列出指定 actor 的 ticket（按 created_at DESC）。
	// 可按 status 过滤（空字符串=不过滤）。
	ListByActor(ctx context.Context, actorUserID string, status string) ([]*Ticket, error)

	// ListByScope 列出关联 scope 的 ticket。
	// 用于 scope 删除时检查 FK 限制（ON DELETE RESTRICT）。
	ListByScope(ctx context.Context, scopeID string) ([]*Ticket, error)
}
