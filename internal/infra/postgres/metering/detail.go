package metering

import (
	"context"
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// Single-row reads mirror ListEvents/ListAggregates: both the primary key and
// organization_id are in the predicate, so a cross-tenant id is indistinguishable
// from a missing one (404, never 403/200).

const queryGetEventByID = `
SELECT id, organization_id, event_type, subject, quantity, unit, idempotency_key, occurred_at, metadata, created_at
FROM guliangiam.metering_events
WHERE id = $1 AND organization_id = $2
`

const queryGetAggregateByID = `
SELECT id, organization_id, period, subject, metric, quantity, unit, calculated_at, created_at
FROM guliangiam.metering_aggregates
WHERE id = $1 AND organization_id = $2
`

func (r Repository) GetEventByID(ctx context.Context, id, organizationID string) (*domain.MeteringEvent, error) {
	var event domain.MeteringEvent
	err := r.db.QueryRow(ctx, queryGetEventByID, id, organizationID).Scan(
		&event.ID, &event.OrganizationID, &event.EventType, &event.Subject, &event.Quantity,
		&event.Unit, &event.IdempotencyKey, &event.OccurredAt, &event.Metadata, &event.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NewNotFound("metering event not found")
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r Repository) GetAggregateByID(ctx context.Context, id, organizationID string) (*domain.MeteringAggregate, error) {
	var aggregate domain.MeteringAggregate
	err := r.db.QueryRow(ctx, queryGetAggregateByID, id, organizationID).Scan(
		&aggregate.ID, &aggregate.OrganizationID, &aggregate.Period, &aggregate.Subject,
		&aggregate.Metric, &aggregate.Quantity, &aggregate.Unit, &aggregate.CalculatedAt, &aggregate.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NewNotFound("metering aggregate not found")
	}
	if err != nil {
		return nil, err
	}
	return &aggregate, nil
}
