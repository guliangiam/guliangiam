package webhookevents

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 webhookevents 存储层。
// 与 internal/infra/postgres/webhookevents.Repository 实现同域接口。
//
// 关键 PG→MySQL 差异处理：
//  1. pgx.ErrNoRows → sql.ErrNoRows
//  2. QueryRow/Query → QueryRowContext/QueryContext
//  3. nullable 字段 *int/*string/*time.Time → sql.NullInt64/sql.NullString/sql.NullTime
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx。
func NewRepository(db mysqlshared.DBTX) Repository { return Repository{db: db} }

// rowsAffected 从 sql.Result 安全提取影响行数，替代 PG 的 commandTag.RowsAffected()。
func rowsAffected(r sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ── domain.Repository 实现 ────────────────────────────────────────

func (r Repository) GetByID(ctx context.Context, id string) (*domain.WebhookEvent, error) {
	return r.getByID(ctx, queryGetByID, id)
}

// GetByIDForOrganization 只在事件父 webhook 属于 organizationID 时返回；
// 跨租户与不存在同为 not found。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.WebhookEvent, error) {
	return r.getByID(ctx, queryGetByIDForOrganization, id, organizationID)
}

func (r Repository) getByID(ctx context.Context, query string, args ...any) (*domain.WebhookEvent, error) {
	row, err := scanWebhookEventRow(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("webhook event not found")
		}
		return nil, err
	}
	return toWebhookEvent(row)
}

// maxBatchInsertRows / maxIDsPerQuery keep each statement's placeholder
// count below MySQL's 65,535 prepared-statement limit (13 columns × 4,000
// rows = 52,000). Package-level vars so tests can exercise the chunking path
// with small inputs.
var (
	maxBatchInsertRows = 4000
	maxIDsPerQuery     = 4000
)

// canonicalIDs normalizes ids to the lowercase form the domain contract
// promises across dialects (PG id::text is lowercase; deterministic delivery
// ids are already lowercase UUIDs) and drops blanks. MySQL CHAR(36) matching
// depends on collation, so binding the canonical form keeps fan-out replay
// checks equivalent to the PG behavior.
func canonicalIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// ListExistingIDs returns the subset of ids that already exist. The id set is
// canonicalized and split into IN-list chunks of ≤ maxIDsPerQuery ids.
func (r Repository) ListExistingIDs(ctx context.Context, ids []string) ([]string, error) {
	normalized := canonicalIDs(ids)
	if len(normalized) == 0 {
		return nil, nil
	}

	found := make([]string, 0, len(normalized))
	for start := 0; start < len(normalized); start += maxIDsPerQuery {
		end := min(start+maxIDsPerQuery, len(normalized))
		chunk := normalized[start:end]
		query := queryListExistingIDsPrefix + strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",") + ")"
		args := make([]any, 0, len(chunk))
		for _, id := range chunk {
			args = append(args, id)
		}
		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			found = append(found, id)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return nil, rowsErr
		}
		rows.Close()
	}
	return found, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.WebhookEvent, int64, error) {
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	limit := mysqlshared.ClampPageSize(q.PageSize)

	var status any
	if q.Status != nil {
		status = *q.Status
	}

	// 参数顺序：webhook_id(×2), event_type(×2), status(×2), organization_id(×2), limit, offset
	// organization_id 为空表示平台全租户作用域（已由 app 层裁决）；非空时经父表谓词绑定，
	// 跨租户父 webhook 直接返回空，不依赖 RLS（MySQL 无 RLS）。
	rows, err := r.db.QueryContext(ctx, queryList,
		q.WebhookID, q.WebhookID,
		q.EventType, q.EventType,
		status, status,
		q.OrganizationID, q.OrganizationID,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.WebhookEvent, 0)
	for rows.Next() {
		row, err := scanWebhookEventRow(rows)
		if err != nil {
			return nil, 0, err
		}
		e, err := toWebhookEvent(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Count: webhook_id(×2), event_type(×2), status(×2), organization_id(×2) = 8 个
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCount,
		q.WebhookID, q.WebhookID,
		q.EventType, q.EventType,
		status, status,
		q.OrganizationID, q.OrganizationID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// webhookEventInsertArgs 与 queryInsert 的 13 个占位符严格同序；
// Create 与 CreateBatch 共用，保证单行/批量写入口径一致。
func webhookEventInsertArgs(e *domain.WebhookEvent) []any {
	var payload json.RawMessage
	if e.Payload != nil {
		payload = e.Payload
	} else {
		payload = json.RawMessage(`{}`)
	}
	var responseCode sql.NullInt64
	if e.ResponseCode != 0 {
		responseCode = sql.NullInt64{Int64: int64(e.ResponseCode), Valid: true}
	}
	var responseBody sql.NullString
	if e.ResponseBody != "" {
		responseBody = sql.NullString{String: e.ResponseBody, Valid: true}
	}
	var idempotencyKey sql.NullString
	if e.IdempotencyKey != "" {
		idempotencyKey = sql.NullString{String: e.IdempotencyKey, Valid: true}
	}
	return []any{
		e.ID, e.WebhookID, e.EventType, payload,
		responseCode, responseBody, e.Status, e.RetryCount,
		nullTime(e.NextRetryAt), idempotencyKey,
		e.CreatedAt, e.UpdatedAt, e.Revision,
	}
}

// CreateBatch writes all events with chunked multi-row INSERTs (each chunk
// ≤ maxBatchInsertRows rows to stay below the placeholder limit). Callers run
// it inside a transaction so the batch commits or rolls back as a unit.
func (r Repository) CreateBatch(ctx context.Context, events []*domain.WebhookEvent) error {
	const columnsPerRow = 13
	rowPlaceholder := "(" + strings.TrimSuffix(strings.Repeat("?,", columnsPerRow), ",") + ")"
	for start := 0; start < len(events); start += maxBatchInsertRows {
		end := min(start+maxBatchInsertRows, len(events))
		chunk := events[start:end]
		values := make([]string, 0, len(chunk))
		args := make([]any, 0, len(chunk)*columnsPerRow)
		for _, e := range chunk {
			values = append(values, rowPlaceholder)
			args = append(args, webhookEventInsertArgs(e)...)
		}
		query := queryInsertBatchPrefix + strings.Join(values, ",")
		if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) Create(ctx context.Context, e *domain.WebhookEvent) error {
	_, err := r.db.ExecContext(ctx, queryInsert, webhookEventInsertArgs(e)...)
	return err
}

func (r Repository) Update(ctx context.Context, e *domain.WebhookEvent) error {
	var responseCode sql.NullInt64
	if e.ResponseCode != 0 {
		responseCode = sql.NullInt64{Int64: int64(e.ResponseCode), Valid: true}
	}
	var responseBody sql.NullString
	if e.ResponseBody != "" {
		responseBody = sql.NullString{String: e.ResponseBody, Valid: true}
	}
	var idempotencyKey sql.NullString
	if e.IdempotencyKey != "" {
		idempotencyKey = sql.NullString{String: e.IdempotencyKey, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, queryUpdate,
		responseCode, responseBody, e.Status, e.RetryCount,
		nullTime(e.NextRetryAt), idempotencyKey,
		e.UpdatedAt, e.Revision, e.ID,
	)
	return err
}

func (r Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryDelete, id)
	return err
}

// 编译期接口断言
var _ domain.Repository = Repository{}
