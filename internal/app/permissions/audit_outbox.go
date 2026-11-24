// audit_outbox.go — 薄 wrapper 委托给 internal/platform/outbox/providers/audit（审计事件唯一写入提供者）。
// 共享实现（单一事实源）在 internal/platform/outbox/providers/audit。
package permissions

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// embedAuditEvent 为 Service 在同一事务内写一条审计 Record + 一条
// OutboxEvent。内部委托给 platform/outbox/providers/audit.EmbedAuditEvent。
func (s *Service) embedAuditEvent(
	tx appuow.UnitOfWork, ctx context.Context,
	ipAddress, userAgent, surface, requestID, traceID string,
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
		ipAddress, userAgent, surface, requestID, traceID,
		orgID, actorID, action, resourceType, resourceID,
		aggregateType, eventType,
		payload,
	)
}
