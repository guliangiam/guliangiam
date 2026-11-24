package outbox

import (
	"context"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements outbox.Repository over PostgreSQL.
//
// LockBatch uses SELECT ... FOR UPDATE SKIP LOCKED to guarantee that multiple
// workers picking from the same table never pull the same row.
//
// MarkDead needs a pgxpool.Pool (shared.DBTX has no Begin) to run the
// dead-letter INSERT + outbox UPDATE atomically. SetPool at wiring time.
type Repository struct {
	db   shared.DBTX
	pool *pgxpool.Pool
}

func NewRepository(db shared.DBTX, pool *pgxpool.Pool) *Repository {
	return &Repository{db: db, pool: pool}
}

// withSystemTx 在系统作用域事务内执行 fn：outbox 表是 FORCE RLS 平台表，
// Worker 无请求租户上下文，直接走 pool 会返回 0 行（DEF-46）且写操作被拒。
func (r *Repository) withSystemTx(ctx context.Context, fn func(db shared.DBTX) error) error {
	if r.pool == nil {
		return fn(r.db)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.current_role', 'super_admin', true), set_config('app.current_organization_id', '', true)`,
	); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// Insert writes one outbox row. Typically called inside the same business
// transaction that writes the fact row — caller is responsible for sharing
// the tx (UoW pattern passes the tx via NewRepository(db=tx)).
func (r *Repository) Insert(ctx context.Context, item *platformoutbox.OutboxItem) error {
	orgID := item.OrganizationID
	payload := item.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	headers := item.Headers
	if len(headers) == 0 {
		headers = []byte("{}")
	}

	_, err := r.db.Exec(ctx, sqlInsert,
		item.ID, orgID, item.AggregateType, item.AggregateID, item.EventType,
		item.EventVersion, payload, headers,
		string(item.Status), item.Attempts, item.MaxAttempts,
		item.AvailableAt,
	)
	return err
}

// LockBatch picks up to limit rows: pending 且到期，或 processing 且租约过期（DEF-32）。
// outbox 是 FORCE RLS 平台表，Worker 无请求租户上下文，必须在系统作用域事务内取件
// （DEF-46）；取件事务在扫描完成即提交，行锁释放但 status=processing + 租约 TTL 兜底。
func (r *Repository) LockBatch(ctx context.Context, limit int, workerID string, lockTTL time.Duration) ([]*platformoutbox.OutboxItem, error) {
	items := make([]*platformoutbox.OutboxItem, 0, limit)
	err := r.withSystemTx(ctx, func(db shared.DBTX) error {
		rows, qerr := db.Query(ctx, sqlLockBatch, limit, workerID, lockTTL.Seconds())
		if qerr != nil {
			return qerr
		}
		defer rows.Close()
		for rows.Next() {
			var row outboxRow
			var lockedBy, lastError *string
			var lockedAtTs, processedAtTs *time.Time
			var availableAt, createdAt, updatedAt time.Time

			if err := rows.Scan(
				&row.ID, &row.OrganizationID, &row.AggregateType, &row.AggregateID, &row.EventType,
				&row.EventVersion, &row.Payload, &row.Headers, &row.Status,
				&row.Attempts, &row.MaxAttempts,
				&availableAt, &lockedBy, &lockedAtTs, &processedAtTs, &lastError,
				&createdAt, &updatedAt, &row.Revision,
			); err != nil {
				return err
			}
			row.AvailableAt = availableAt
			row.LockedBy = lockedBy
			row.LockedAt = lockedAtTs
			row.ProcessedAt = processedAtTs
			row.LastError = lastError
			row.CreatedAt = createdAt
			row.UpdatedAt = updatedAt

			items = append(items, toOutboxItem(row))
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// MarkSuccess sets status=succeeded + processed_at.
func (r *Repository) MarkSuccess(ctx context.Context, id string, processedAt time.Time) error {
	return r.withSystemTx(ctx, func(db shared.DBTX) error {
		_, err := db.Exec(ctx, sqlMarkSuccess, id, processedAt)
		return err
	})
}

// MarkRetry increments attempts by 1, sets available_at = new time, goes back to pending.
func (r *Repository) MarkRetry(ctx context.Context, id, errMsg string, now, availableAt time.Time) error {
	_ = now
	return r.withSystemTx(ctx, func(db shared.DBTX) error {
		_, err := db.Exec(ctx, sqlMarkRetry, id, errMsg, availableAt)
		return err
	})
}

// MarkDead writes dead-letter row + flips outbox.status=dead atomically via a pgx transaction.
func (r *Repository) MarkDead(ctx context.Context, id, errMsg string, deadAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.current_role', 'super_admin', true), set_config('app.current_organization_id', '', true)`,
	); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx) // no-op if Commit succeeded
	}()

	if _, err = tx.Exec(ctx, sqlMarkDead_Letter, id, errMsg, deadAt); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, sqlMarkDead_Outbox, id, errMsg); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ── Domain adapter ──────────────────────────────────────────────────────
// 让 infra Repository 同时满足 domainoutbox.Repository 接口，避免 app 层
// 感知 platform/outbox.OutboxItem 这种基础设施类型。

// InsertDomain 是 domain 层调用的入口。把 domain.OutboxEvent 转成
// platform.OutboxItem 再走同一条 INSERT 路径。
func (r *Repository) Enqueue(ctx context.Context, ev *domainoutbox.OutboxEvent) error {
	item := &platformoutbox.OutboxItem{
		ID:             ev.ID,
		OrganizationID: ev.OrganizationID,
		AggregateType:  ev.AggregateType,
		AggregateID:    ev.AggregateID,
		EventType:      ev.EventType,
		EventVersion:   ev.EventVersion,
		Payload:        ev.Payload,
		Headers:        ev.Headers,
		Status:         platformoutbox.StatusPending,
		Attempts:       0,
		MaxAttempts:    ev.MaxAttempts,
		AvailableAt:    ev.AvailableAt,
	}
	if item.MaxAttempts <= 0 {
		item.MaxAttempts = 5
	}
	if item.AvailableAt.IsZero() {
		item.AvailableAt = time.Now()
	}
	return r.Insert(ctx, item)
}

// EnqueueBatch 批量 domain → platform 转换后一次 INSERT。
func (r *Repository) EnqueueBatch(ctx context.Context, evs []*domainoutbox.OutboxEvent) error {
	for _, ev := range evs {
		if err := r.Enqueue(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

// 编译期断言：infra Repository 实现 domainoutbox.Repository
var _ domainoutbox.Repository = (*Repository)(nil)
