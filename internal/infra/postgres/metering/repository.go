package metering

import (
	"context"
	"fmt"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

var _ domain.Repository = Repository{}

// ─── Events ──────────────────────────────────────────────────────────────

func (r Repository) ListEvents(ctx context.Context, q domain.ListEventsQuery) ([]*domain.MeteringEvent, int64, error) {
	args := []any{q.OrganizationID}
	extra := ""
	if q.EventType != nil && *q.EventType != "" {
		extra, args = shared.AppendParamCondition(extra, "event_type", *q.EventType, args)
	}
	if q.Subject != nil && *q.Subject != "" {
		extra, args = shared.AppendParamCondition(extra, "subject", *q.Subject, args)
	}
	if q.From != nil {
		args = append(args, *q.From)
		extra += fmt.Sprintf(" AND occurred_at >= $%d", len(args))
	}
	if q.To != nil {
		args = append(args, *q.To)
		extra += fmt.Sprintf(" AND occurred_at <= $%d", len(args))
	}

	countSQL := queryCountEvents + extra
	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := shared.PageOffset(q.Page, q.PageSize)
	selectSQL := queryListEventsSelect + extra + " ORDER BY occurred_at DESC"
	selectSQL += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	rows, err := r.db.Query(ctx, selectSQL, append(append([]any{}, args...), q.PageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.MeteringEvent, 0, q.PageSize)
	for rows.Next() {
		var m domain.MeteringEvent
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.EventType, &m.Subject, &m.Quantity, &m.Unit,
			&m.IdempotencyKey, &m.OccurredAt, &m.Metadata, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &m)
	}
	return items, total, rows.Err()
}

func (r Repository) InsertEvent(ctx context.Context, ev *domain.MeteringEvent) (bool, error) {
	cmd, err := r.db.Exec(ctx, queryInsertEvent,
		ev.ID, ev.OrganizationID, ev.EventType, ev.Subject, ev.Quantity, ev.Unit,
		ev.IdempotencyKey, ev.OccurredAt, ev.Metadata)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

func (r Repository) IngestBatch(ctx context.Context, events []*domain.MeteringEvent) (inserted, duplicates int, err error) {
	inserted, dupes := 0, 0
	for _, ev := range events {
		ok, err := r.InsertEvent(ctx, ev)
		if err != nil {
			return inserted, dupes, err
		}
		if ok {
			inserted++
		} else {
			dupes++
		}
	}
	return inserted, dupes, nil
}

// ─── Combos ──────────────────────────────────────────────────────────────

func (r Repository) ScanDistinctCombos(ctx context.Context, since time.Time) ([]domain.Combo, error) {
	rows, err := r.db.Query(ctx, queryScanDistinctCombos, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Combo
	for rows.Next() {
		var c domain.Combo
		if err := rows.Scan(&c.OrganizationID, &c.Subject, &c.EventType); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ─── Aggregates ──────────────────────────────────────────────────────────

func (r Repository) ListAggregates(ctx context.Context, q domain.ListAggregatesQuery) ([]*domain.MeteringAggregate, int64, error) {
	args := []any{q.OrganizationID}
	extra := ""
	if q.Period != nil && *q.Period != "" {
		extra, args = shared.AppendParamCondition(extra, "period", string(*q.Period), args)
	}
	if q.Subject != nil && *q.Subject != "" {
		extra, args = shared.AppendParamCondition(extra, "subject", *q.Subject, args)
	}
	if q.Metric != nil && *q.Metric != "" {
		extra, args = shared.AppendParamCondition(extra, "metric", *q.Metric, args)
	}

	countSQL := queryCountAggregates + extra
	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := shared.PageOffset(q.Page, q.PageSize)
	selectSQL := queryListAggregatesSelect + extra + " ORDER BY period DESC"
	selectSQL += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	rows, err := r.db.Query(ctx, selectSQL, append(append([]any{}, args...), q.PageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.MeteringAggregate, 0, q.PageSize)
	for rows.Next() {
		var a domain.MeteringAggregate
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Period, &a.Subject, &a.Metric,
			&a.Quantity, &a.Unit, &a.CalculatedAt, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &a)
	}
	return items, total, rows.Err()
}

func (r Repository) UpsertAggregate(ctx context.Context, agg *domain.MeteringAggregate) error {
	_, err := r.db.Exec(ctx, queryUpsertAggregate,
		agg.ID, agg.OrganizationID, string(agg.Period), agg.Subject, agg.Metric,
		agg.Quantity, agg.Unit)
	return err
}

func (r Repository) RefreshAggregate(ctx context.Context, cmd domain.RefreshAggregateCommand) (*domain.MeteringAggregate, error) {
	// id is the row primary key, not the upsert key: idempotency comes from
	// uq_metering_aggregates_org_period_subject_metric + ON CONFLICT DO UPDATE,
	// which keeps the id of an already-existing aggregate row.
	aggregateID, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return nil, err
	}

	var a domain.MeteringAggregate
	args := []any{
		cmd.OrganizationID,
		cmd.Subject,
		cmd.Metric,
		string(cmd.Period),
		aggregateID,
		cmd.PeriodStart,
		cmd.PeriodEnd,
	}
	err = r.db.QueryRow(ctx, queryRefreshAggregate, args...).Scan(
		&a.ID, &a.OrganizationID, &a.Period, &a.Subject, &a.Metric, &a.Quantity,
		&a.Unit, &a.CalculatedAt, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
