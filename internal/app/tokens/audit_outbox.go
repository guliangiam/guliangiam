// audit_outbox.go — 薄 wrapper 委托给 internal/platform/outbox/providers/audit（审计事件唯一写入提供者）。
// 共享实现（单一事实源）在 internal/platform/outbox/providers/audit。
package tokens

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// auditPayloadKeyOrgID 是撤销类审计事件载荷里的组织键名。三条撤销路径
// （单个 / 批量 / 删除）写的是同一组键，字面量分散在三处时键名漂移在 diff 里
// 看不出来，因此收敛为一个常量（goconst 也正是以 3 次出现为阈值）。
const auditPayloadKeyOrgID = "orgId"

// embedAuditEvent 为 Service 在同一事务内写一条审计 Record + 一条
// OutboxEvent。内部委托给 platform/outbox/providers/audit.EmbedAuditEvent。
func (s *Service) embedAuditEvent(
	tx appuow.UnitOfWork, ctx context.Context,
	orgID, actorID, action, resourceType, resourceID,
	aggregateType, eventType string,
	payload map[string]any,
) error {
	// BE-D-13：显式 actor 为空时回落到 transport 注入的可信操作者。
	if actorID == "" {
		actorID = actorIDFromContext(ctx)
	}
	return audit.EmbedAuditEvent(
		s.Clock, tx, ctx,
		"", "", "", "", "",
		orgID, actorID, action, resourceType, resourceID,
		aggregateType, eventType,
		payload,
	)
}
