package outbox

import (
	"context"
	"strings"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

// Repository 是 MySQL 8.0.1+ 实现的 outbox 存储层。
//
// 关键 PG→MySQL 差异处理：
//  1. FOR UPDATE SKIP LOCKED（MySQL 8.0.1+ 原生支持）
//  2. LockBatch 分两步走（先 SELECT picked IDs → UPDATE → SELECT full rows）
//     PG 用 CTE + RETURNING 一步完成，MySQL 分三步（避免 CTE + RETURNING 不兼容）
//  3. MarkDead 内部事务用 mysqlshared.Beginner 断言（替代 pgxpool.Pool.Begin）
//  4. ON CONFLICT (id) DO NOTHING → ON DUPLICATE KEY UPDATE id = id（MySQL 等价写法）
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx。
func NewRepository(db mysqlshared.DBTX) *Repository {
	return &Repository{db: db}
}

var _ platformoutbox.Repository = (*Repository)(nil)
var _ domainoutbox.Repository = (*Repository)(nil)

// ── platform/outbox.Repository ────────────────────────────────────────

// Insert 写入一个 outbox 行（业务事务内调用，tx 由 UoW 外层共享）。
func (r *Repository) Insert(ctx context.Context, item *platformoutbox.OutboxItem) error {
	payload := item.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	headers := item.Headers
	if len(headers) == 0 {
		headers = []byte("{}")
	}

	_, err := r.db.ExecContext(ctx, sqlInsert,
		item.ID, item.OrganizationID, item.AggregateType, item.AggregateID, item.EventType,
		item.EventVersion, payload, headers,
		string(item.Status), item.Attempts, item.MaxAttempts,
		item.AvailableAt,
	)
	return err
}

// LockBatch 租约取件：先 SELECT picked IDs（FOR UPDATE SKIP LOCKED），再 UPDATE 加锁，最后 SELECT 全量返回。
// PG 版用 CTE + RETURNING 一步到位；MySQL 版因为没有 RETURNING，拆成三步。
func (r *Repository) LockBatch(ctx context.Context, limit int, workerID string, lockTTL time.Duration) ([]*platformoutbox.OutboxItem, error) {
	// Step 1: 选出 picked IDs（FOR UPDATE SKIP LOCKED 让多 worker 不重复取）
	pickRows, err := r.db.QueryContext(ctx, sqlLockBatch_Pick, limit)
	if err != nil {
		return nil, err
	}
	var pickedIDs []string
	for pickRows.Next() {
		var id string
		if err := pickRows.Scan(&id); err != nil {
			pickRows.Close()
			return nil, err
		}
		pickedIDs = append(pickedIDs, id)
	}
	pickRows.Close()
	if err := pickRows.Err(); err != nil {
		return nil, err
	}
	if len(pickedIDs) == 0 {
		return nil, nil // 没有待处理的 outbox 行
	}

	// Step 2: 用 IN (id1, id2, ...) 更新 + 取回（MySQL driver 不支持 $1 IN ($1, $2, $3) 重复绑定，必须手动展开）
	inClause := buildIN(pickedIDs)

	updateSQL := strings.Replace(sqlLockBatch_Update, "__PICKED_IDS__", inClause, 1)
	updateArgs := []any{workerID}
	for _, id := range pickedIDs {
		updateArgs = append(updateArgs, id)
	}

	if _, err := r.db.ExecContext(ctx, updateSQL, updateArgs...); err != nil {
		return nil, err
	}

	// Step 3: SELECT 已锁行（列必须严格按 sqlLockBatch_Select 的顺序 Scan）
	selectSQL := strings.Replace(sqlLockBatch_Select, "__PICKED_IDS__", inClause, 1)
	selectArgs := make([]any, 0, len(pickedIDs))
	for _, id := range pickedIDs {
		selectArgs = append(selectArgs, id)
	}

	rows, err := r.db.QueryContext(ctx, selectSQL, selectArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*platformoutbox.OutboxItem
	for rows.Next() {
		var row outboxRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.AggregateType, &row.AggregateID, &row.EventType,
			&row.EventVersion, &row.Payload, &row.Headers, &row.Status,
			&row.Attempts, &row.MaxAttempts,
			&row.AvailableAt, &row.LockedBy, &row.LockedAt, &row.ProcessedAt, &row.LastError,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, err
		}
		items = append(items, toOutboxItem(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// MarkSuccess 标记投递成功。
func (r *Repository) MarkSuccess(ctx context.Context, id string, processedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, sqlMarkSuccess, processedAt, id)
	return err
}

// MarkRetry 递增 attempts + 指数退避 available_at。
func (r *Repository) MarkRetry(ctx context.Context, id, errMsg string, _, availableAt time.Time) error {
	_, err := r.db.ExecContext(ctx, sqlMarkRetry, errMsg, availableAt, id)
	return err
}

// MarkDead 原子写入 dead_letters + outbox.status=dead。
// PG 版用 pgxpool.Begin()；MySQL 版用 mysqlshared.Beginner 断言 *sql.DB 有 BeginTx()。
func (r *Repository) MarkDead(ctx context.Context, id, errMsg string, deadAt time.Time) error {
	// 检查是否能开事务（*sql.DB 可以；*sql.Tx 不可以嵌套 BeginTx）
	beginner, ok := r.db.(mysqlshared.Beginner)
	if !ok {
		// 已经在事务内（来自 WithinTransaction），直接顺序执行——事务保证原子性
		if _, err := r.db.ExecContext(ctx, sqlMarkDead_Letter, errMsg, deadAt, id); err != nil {
			return err
		}
		_, err := r.db.ExecContext(ctx, sqlMarkDead_Outbox, errMsg, id)
		return err
	}

	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, sqlMarkDead_Letter, errMsg, deadAt, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, sqlMarkDead_Outbox, errMsg, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ── domain/outbox.Repository ──────────────────────────────────────────

// Enqueue 把 domain.OutboxEvent 转成 platform.OutboxItem 后 INSERT。
func (r *Repository) Enqueue(ctx context.Context, ev *domainoutbox.OutboxEvent) error {
	maxAttempts := ev.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	availableAt := ev.AvailableAt
	if availableAt.IsZero() {
		availableAt = time.Now()
	}

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
		MaxAttempts:    maxAttempts,
		AvailableAt:    availableAt,
	}
	return r.Insert(ctx, item)
}

// EnqueueBatch 逐个 Enqueue（批量 INSERT 优化可后续做）。
func (r *Repository) EnqueueBatch(ctx context.Context, evs []*domainoutbox.OutboxEvent) error {
	for _, ev := range evs {
		if err := r.Enqueue(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

// ── 辅助 ──────────────────────────────────────────────────────────────

// buildIN 生成 "?, ?, ?, ?" 字符串，让 MySQL driver 按顺序绑定 pickedIDs 列表。
func buildIN(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	// 构建 ?, ?, ?, ?
	placeholders := make([]string, len(ids))
	for i := range ids {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}
