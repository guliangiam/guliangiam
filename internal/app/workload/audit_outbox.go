package workload

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

func (s *Service) embedAuditEvent(
	tx appuow.UnitOfWork,
	ctx context.Context,
	orgID, action, aggregateType, aggregateID string,
	payload map[string]any,
) error {
	clockProvider := s.Clock
	if clockProvider == nil {
		clockProvider = clock.SystemClock{}
	}
	return audit.EmbedAuditEvent(
		clockProvider, tx, ctx,
		"", "", "management", "", "",
		orgID, "", action, aggregateType, aggregateID,
		aggregateType, action, payload,
	)
}
