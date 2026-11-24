package metering

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// Repository 是 MySQL 8.4+ 实现的 metering 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// ── domain.Repository 实现 ────────────────────────────────────────

// ListEvents 分页列出计量事件（动态 SQL 拼接 WHERE 条件）。
func (r Repository) ListEvents(ctx context.Context, q domain.ListEventsQuery) ([]*domain.MeteringEvent, int64, error) {
	selectSQL, countSQL, args := buildListEventsSQL(q, false)

	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	pagedArgs := append(append([]any{}, args...), limit, offset)
	selectSQL += paginationSuffix

	rows, err := r.db.QueryContext(ctx, selectSQL, pagedArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.MeteringEvent, 0, limit)
	for rows.Next() {
		var m domain.MeteringEvent
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.EventType, &m.Subject, &m.Quantity, &m.Unit,
			&m.IdempotencyKey, &m.OccurredAt, &m.Metadata, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// InsertEvent 插入一条计量事件，重复 idempotency_key 时返回 false（INSERT IGNORE）。
func (r Repository) InsertEvent(ctx context.Context, ev *domain.MeteringEvent) (bool, error) {
	res, err := r.db.ExecContext(ctx, queryInsertEvent,
		ev.ID, ev.OrganizationID, ev.EventType, ev.Subject, ev.Quantity, ev.Unit,
		ev.IdempotencyKey, ev.OccurredAt, ev.Metadata,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// IngestBatch 批量插入计量事件，返回 (inserted, duplicates)。
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

// ScanDistinctCombos 取最近时间窗内 distinct (org, subject, event_type) 组合。
func (r Repository) ScanDistinctCombos(ctx context.Context, since time.Time) ([]domain.Combo, error) {
	rows, err := r.db.QueryContext(ctx, queryScanDistinctCombos, since)
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

// ListAggregates 分页列出聚合记录（动态 SQL 拼接 WHERE 条件）。
func (r Repository) ListAggregates(ctx context.Context, q domain.ListAggregatesQuery) ([]*domain.MeteringAggregate, int64, error) {
	selectSQL, countSQL, args := buildListAggregatesSQL(q, false)

	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	pagedArgs := append(append([]any{}, args...), limit, offset)
	selectSQL += paginationSuffix

	rows, err := r.db.QueryContext(ctx, selectSQL, pagedArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.MeteringAggregate, 0, limit)
	for rows.Next() {
		var a domain.MeteringAggregate
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Period, &a.Subject, &a.Metric,
			&a.Quantity, &a.Unit, &a.CalculatedAt, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// UpsertAggregate 插入或更新聚合记录（ON DUPLICATE KEY UPDATE）。
func (r Repository) UpsertAggregate(ctx context.Context, agg *domain.MeteringAggregate) error {
	_, err := r.db.ExecContext(ctx, queryUpsertAggregate,
		agg.ID, agg.OrganizationID, string(agg.Period), agg.Subject, agg.Metric,
		agg.Quantity, agg.Unit,
	)
	return err
}

// RefreshAggregate 按需刷新指定维度的聚合（先 Exec 再 SELECT 回读 — MySQL 无 RETURNING）。
// 两种模式同 PG：PeriodStart/PeriodEnd 显式时间边界 or DATE_FORMAT fallback。
func (r Repository) RefreshAggregate(ctx context.Context, cmd domain.RefreshAggregateCommand) (*domain.MeteringAggregate, error) {
	// id is the row primary key, not the upsert key: idempotency comes from
	// uq_metering_aggregates_org_period_subject_metric + ON DUPLICATE KEY UPDATE,
	// which keeps the id of an already-existing aggregate row.
	newID, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return nil, err
	}
	// SQL 占位符：1-3 WHERE, 4-7 时间边界分支, 8-10 IS NULL + DATE_FORMAT fallback, 11-15 INSERT
	args := []any{
		cmd.OrganizationID,        // 1
		cmd.Subject,               // 2
		cmd.Metric,                // 3
		cmd.PeriodStart,           // 4
		cmd.PeriodEnd,             // 5
		cmd.PeriodStart,           // 6
		cmd.PeriodEnd,             // 7
		cmd.PeriodStart,           // 8 (IS NULL check)
		cmd.PeriodEnd,             // 9 (IS NULL check)
		string(cmd.Period),        // 10 (DATE_FORMAT fallback)
		newID, cmd.OrganizationID, // 11, 12
		string(cmd.Period), cmd.Subject, cmd.Metric, // 13, 14, 15
	}
	_, err = r.db.ExecContext(ctx, queryRefreshAggregate, args...)
	if err != nil {
		return nil, err
	}

	var a domain.MeteringAggregate
	err = r.db.QueryRowContext(ctx, queryRefreshAggregateSelect,
		cmd.OrganizationID, string(cmd.Period), cmd.Subject, cmd.Metric,
	).Scan(&a.ID, &a.OrganizationID, &a.Period, &a.Subject, &a.Metric,
		&a.Quantity, &a.Unit, &a.CalculatedAt, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
