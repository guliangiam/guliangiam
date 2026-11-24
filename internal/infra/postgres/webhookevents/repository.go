package webhookevents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id string) (*domain.WebhookEvent, error) {
	return r.getByID(ctx, QueryGetByID, id)
}

// GetByIDForOrganization binds the event read to the tenant that owns its
// parent webhook. A foreign or missing id is the same NotFound.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.WebhookEvent, error) {
	return r.getByID(ctx, QueryGetByIDForOrganization, id, organizationID)
}

func (r Repository) getByID(ctx context.Context, query string, args ...any) (*domain.WebhookEvent, error) {
	e, err := scanWebhookEvent(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("webhook event not found")
		}
		return nil, err
	}
	return e, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.WebhookEvent, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	// Param order: webhook_id, event_type, status, organization_id, limit, offset.
	// organization_id is a no-op when empty (platform all-tenant scope, already
	// gated by the use case); otherwise QueryList binds it through the parent
	// webhook, so a cross-tenant parent yields no rows.
	rows, err := r.db.Query(ctx, QueryList, q.WebhookID, q.EventType, q.Status, q.OrganizationID, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.WebhookEvent, 0)
	for rows.Next() {
		e, err := scanWebhookEvent(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.WebhookID, q.EventType, q.Status, q.OrganizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// maxBatchInsertRows keeps one statement's bind-parameter count (13 columns
// per row → 52,000) below the 65,535 limit shared by the PG extended protocol
// and MySQL prepared statements. Package-level var so tests can exercise the
// chunking path with small inputs.
var maxBatchInsertRows = 4000

// canonicalIDs normalizes ids to the lowercase form both dialects return
// (PG id::text is lowercase; deterministic delivery ids are already lowercase
// UUIDs) and drops blanks. See the domain Repository contract.
func canonicalIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// ListExistingIDs returns the subset of ids that already exist. PG binds the
// id set as a single uuid[] parameter, so no IN-list chunking is needed; the
// cast normalizes case and id::text returns lowercase.
func (r Repository) ListExistingIDs(ctx context.Context, ids []string) ([]string, error) {
	normalized := canonicalIDs(ids)
	if len(normalized) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, QueryListExistingIDs, normalized)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := make([]string, 0, len(normalized))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		found = append(found, id)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return found, nil
}

func (r Repository) Create(ctx context.Context, e *domain.WebhookEvent) error {
	payload, responseCode, responseBody, idempotencyKey := webhookEventInsertDerived(e)
	// 参数与 QueryInsert 的 $1..$13 严格同序；显式传参供 check-sql-args 静态核对。
	_, err := r.db.Exec(ctx, QueryInsert,
		e.ID, e.WebhookID, e.EventType, payload, responseCode, responseBody,
		e.Status, e.RetryCount, e.NextRetryAt, idempotencyKey,
		e.CreatedAt, e.UpdatedAt, e.Revision,
	)
	return err
}

// CreateBatch writes all events with chunked multi-row INSERTs (placeholders
// restart per statement, each chunk ≤ maxBatchInsertRows rows). Callers run
// it inside a transaction so the batch commits or rolls back as a unit.
func (r Repository) CreateBatch(ctx context.Context, events []*domain.WebhookEvent) error {
	for start := 0; start < len(events); start += maxBatchInsertRows {
		end := min(start+maxBatchInsertRows, len(events))
		if err := r.createBatchChunk(ctx, events[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) createBatchChunk(ctx context.Context, events []*domain.WebhookEvent) error {
	const columnsPerRow = 13
	values := make([]string, 0, len(events))
	args := make([]any, 0, len(events)*columnsPerRow)
	for i, e := range events {
		base := i * columnsPerRow
		placeholders := make([]string, 0, columnsPerRow)
		for col := 1; col <= columnsPerRow; col++ {
			p := fmt.Sprintf("$%d", base+col)
			if col == 1 || col == 2 {
				p += "::uuid"
			}
			placeholders = append(placeholders, p)
		}
		values = append(values, "("+strings.Join(placeholders, ",")+")")
		args = append(args, webhookEventInsertArgs(e)...)
	}
	// 单行占位符子句被整体替换为多行 $N 子句（columnsPerRow 编译期常量 × 行号生成），
	// 数据一律经 pgx args 绑定；动态点登记于 contracts/security/sql-injection-registry.json。
	query := strings.Replace(queryInsertWebhookEventsBatch, webhookEventValuesRow, strings.Join(values, ","), 1)
	_, err := r.db.Exec(ctx, query, args...)
	return err
}

// webhookEventValuesRow 是批量 INSERT 的单行占位符子句（与 QueryInsert 的 13 列
// 严格同序）。模板保持可独立 PREPARE 的完整单行语句，运行期仅替换该子句。
const webhookEventValuesRow = `($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`

const queryInsertWebhookEventsBatch = `INSERT INTO "guliangiam".webhook_events (id, webhook_id, event_type, payload, response_code, response_body, status, retry_count, next_retry_at, idempotency_key, created_at, updated_at, revision) VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`

// webhookEventInsertDerived 归一化可空插入字段（payload 默认 {}、0/空串转 NULL），
// 供单条 Create 与批量 webhookEventInsertArgs 共用同一套规则。
func webhookEventInsertDerived(e *domain.WebhookEvent) (payload json.RawMessage, responseCode *int, responseBody *string, idempotencyKey interface{}) {
	if e.Payload != nil {
		payload = e.Payload
	} else {
		payload = json.RawMessage(`{}`)
	}
	if e.ResponseCode != 0 {
		responseCode = &e.ResponseCode
	}
	if e.ResponseBody != "" {
		responseBody = &e.ResponseBody
	}
	if e.IdempotencyKey != "" {
		idempotencyKey = e.IdempotencyKey
	}
	return payload, responseCode, responseBody, idempotencyKey
}

// webhookEventInsertArgs 与 QueryInsert 的 13 个占位符严格同序。
func webhookEventInsertArgs(e *domain.WebhookEvent) []any {
	payload, responseCode, responseBody, idempotencyKey := webhookEventInsertDerived(e)
	return []any{
		e.ID, e.WebhookID, e.EventType, payload, responseCode, responseBody,
		e.Status, e.RetryCount, e.NextRetryAt, idempotencyKey,
		e.CreatedAt, e.UpdatedAt, e.Revision,
	}
}

func (r Repository) Update(ctx context.Context, e *domain.WebhookEvent) error {
	var responseCode *int
	if e.ResponseCode != 0 {
		responseCode = &e.ResponseCode
	}
	var responseBody *string
	if e.ResponseBody != "" {
		responseBody = &e.ResponseBody
	}
	var idempotencyKey interface{}
	if e.IdempotencyKey != "" {
		idempotencyKey = e.IdempotencyKey
	}
	_, err := r.db.Exec(ctx, QueryUpdate, responseCode, responseBody, e.Status, e.RetryCount, e.NextRetryAt, idempotencyKey, e.UpdatedAt, e.Revision, e.ID)
	return err
}

func (r Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, QueryDelete, id)
	return err
}

var _ domain.Repository = Repository{}
