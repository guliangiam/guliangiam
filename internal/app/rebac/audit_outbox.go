// audit_outbox.go — 薄 wrapper 委托给 internal/platform/outbox/providers/audit（审计事件唯一写入提供者）。
// 共享实现（单一事实源）在 internal/platform/outbox/providers/audit。
package rebac

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// embedAuditEvent 为 Service 在同一事务内写一条审计 Record + 一条
// OutboxEvent。内部委托给 platform/outbox/providers/audit.EmbedAuditEvent。
func (s *Service) embedAuditEvent(
	tx appuow.UnitOfWork, ctx context.Context,
	orgID, action, resourceType, resourceID,
	aggregateType, eventType string,
	payload map[string]any,
) error {
	return audit.EmbedAuditEvent(
		s.Clock, tx, ctx,
		"", "", "", "", "",
		// BE-D-13：actor 取 transport 注入的可信操作者（无会话时为空）。
		orgID, actorIDFromContext(ctx), action, resourceType, resourceID,
		aggregateType, eventType,
		payload,
	)
}
