package metering

import (
	"context"
	"database/sql"
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// MySQL mirror of the PostgreSQL single-row reads (ADR-023). The predicates are
// the same — id plus organization_id — so tenant isolation does not depend on a
// database-specific RLS policy.

const queryGetEventByID = `
SELECT id, organization_id, event_type, subject, quantity, unit, idempotency_key, occurred_at, metadata, created_at
FROM ` + btEvents + `
WHERE id = ? AND organization_id = ?
`

const queryGetAggregateByID = `
SELECT id, organization_id, period, subject, metric, quantity, unit, calculated_at, created_at
FROM ` + btAggregates + `
WHERE id = ? AND organization_id = ?
`

func (r Repository) GetEventByID(ctx context.Context, id, organizationID string) (*domain.MeteringEvent, error) {
	var event domain.MeteringEvent
	err := r.db.QueryRowContext(ctx, queryGetEventByID, id, organizationID).Scan(
		&event.ID, &event.OrganizationID, &event.EventType, &event.Subject, &event.Quantity,
		&event.Unit, &event.IdempotencyKey, &event.OccurredAt, &event.Metadata, &event.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NewNotFound("metering event not found")
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r Repository) GetAggregateByID(ctx context.Context, id, organizationID string) (*domain.MeteringAggregate, error) {
	var aggregate domain.MeteringAggregate
	err := r.db.QueryRowContext(ctx, queryGetAggregateByID, id, organizationID).Scan(
		&aggregate.ID, &aggregate.OrganizationID, &aggregate.Period, &aggregate.Subject,
		&aggregate.Metric, &aggregate.Quantity, &aggregate.Unit, &aggregate.CalculatedAt, &aggregate.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NewNotFound("metering aggregate not found")
	}
	if err != nil {
		return nil, err
	}
	return &aggregate, nil
}
