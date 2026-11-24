package scim

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// embedAuditEvent writes one audit record + outbox event inside the caller's
// transaction (single writer: platform/outbox/providers/audit).
func (s Service) embedAuditEvent(
	tx appuow.UnitOfWork, ctx context.Context,
	orgID, actorID, action, resourceType, resourceID, aggregateType, eventType string,
	payload map[string]any,
) error {
	return audit.EmbedAuditEvent(
		s.Clock, tx, ctx,
		"", "", "", "", "",
		orgID, actorID, action, resourceType, resourceID, aggregateType, eventType,
		payload,
	)
}
