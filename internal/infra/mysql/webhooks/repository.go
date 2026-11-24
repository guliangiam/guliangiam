package webhooks

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 webhooks 存储层。
// 与 internal/infra/postgres/webhooks.Repository 实现同域接口。
//
// 关键 PG→MySQL 差异处理：
//  1. guliangiam.resolve_org_id() → 直接传 organization_id（MySQL 无 schema-qualified 函数）
//  2. events @> jsonb → JSON_CONTAINS(events, ?)（PG JSONB containment → MySQL 等价函数）
//  3. RETURNING revision → UPDATE + SELECT revision（两步替代 PG 原子 RETURNING）
//  4. pgx.ErrNoRows → sql.ErrNoRows
//  5. QueryRow / Query → QueryRowContext / QueryContext
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

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

// scannable 是 sql.Row 和 sql.Rows 的公共接口。
type scannable interface {
	Scan(dest ...any) error
}

// scanWebhookRow 扫描 webhooks 单表查询。
// 列顺序必须与 sql.go 各 SELECT 严格一致（30 列）。
func scanWebhookRow(s scannable) (webhookRow, error) {
	var row webhookRow
	err := s.Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
		&row.URL, &row.Method, &row.Headers, &row.Events,
		&row.IsEnabled, &row.Secret, &row.RetryCount, &row.TimeoutMs,
		&row.Organization, &row.ContentType, &row.CustomBody, &row.CustomHeaders,
		&row.CertID, &row.IsError, &row.ErrorText,
		&row.TokenFields, &row.ObjectFields, &row.IsUserExtended,
		&row.SingleOrgOnly, &row.MaxRetries, &row.RetryInterval, &row.UseExponentialBackoff,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	return row, err
}

// ── domain.Repository 实现 ────────────────────────────────────────

// GetByID 按主键取 webhook。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Webhook, error) {
	return r.getByID(ctx, queryGetWebhookByID, id)
}

// GetByIDForOrganization 只在 webhook 属于 organizationID 时返回；
// 跨租户与不存在同为 not found。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Webhook, error) {
	return r.getByID(ctx, queryGetWebhookByIDForOrganization, id, organizationID)
}

func (r Repository) getByID(ctx context.Context, query string, args ...any) (*domain.Webhook, error) {
	row, err := scanWebhookRow(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("webhook not found")
		}
		return nil, err
	}

	webhook, err := toWebhook(row)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode webhook")
	}
	return webhook, nil
}

// List 分页列出 webhooks。
// SQL 占位符展开：
//
//	organization_id × 2, keyword × 4, is_enabled × 2, method × 2, limit, offset
//
// 共 12 个参数；Count 同构无 limit/offset = 10 个。
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Webhook, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	var isEnabled any
	if query.IsEnabled != nil {
		isEnabled = *query.IsEnabled
	}

	rows, err := r.db.QueryContext(ctx, queryListWebhooks,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
		query.Method, query.Method,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Webhook, 0)
	for rows.Next() {
		row, err := scanWebhookRow(rows)
		if err != nil {
			return nil, 0, err
		}
		webhook, err := toWebhook(row)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "decode webhook")
		}
		items = append(items, webhook)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Count: organization_id(×2), keyword(×4), is_enabled(×2), method(×2) = 10 个
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountWebhooks,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
		query.Method, query.Method,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Create 插入 webhook。
func (r Repository) Create(ctx context.Context, webhook *domain.Webhook) error {
	events, err := mysqlshared.MarshalJSON(webhook.Events)
	if err != nil {
		return err
	}
	tokenFields, err := mysqlshared.MarshalJSON(webhook.TokenFields)
	if err != nil {
		return err
	}
	objectFields, err := mysqlshared.MarshalJSON(webhook.ObjectFields)
	if err != nil {
		return err
	}

	if err := encryptWebhookSecrets(webhook); err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryInsertWebhook,
		webhook.ID, webhook.OrganizationID, webhook.Name, webhook.DisplayName,
		webhook.URL, webhook.Method, webhook.Headers, events,
		webhook.IsEnabled, webhook.Secret, webhook.RetryCount, webhook.TimeoutMs,
		webhook.Organization, webhook.ContentType, webhook.CustomBody, webhook.CustomHeaders,
		webhook.CertID, webhook.IsError, webhook.ErrorText,
		tokenFields, objectFields, webhook.IsUserExtended,
		webhook.SingleOrgOnly, webhook.MaxRetries, webhook.RetryInterval, webhook.UseExponentialBackoff,
		webhook.CreatedAt, webhook.UpdatedAt, webhook.Revision,
	)
	return err
}

// Update 乐观锁更新。
// PG 版用 RETURNING revision 一步到位；MySQL 版拆成两步：
//  1. UPDATE ... WHERE id = ? AND revision = ? → 判断影响行数
//  2. SELECT revision FROM ... WHERE id = ? → 回读新 revision
func (r Repository) Update(ctx context.Context, webhook *domain.Webhook) error {
	events, err := mysqlshared.MarshalJSON(webhook.Events)
	if err != nil {
		return err
	}
	tokenFields, err := mysqlshared.MarshalJSON(webhook.TokenFields)
	if err != nil {
		return err
	}
	objectFields, err := mysqlshared.MarshalJSON(webhook.ObjectFields)
	if err != nil {
		return err
	}

	if err := encryptWebhookSecrets(webhook); err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx, queryUpdateWebhook,
		webhook.DisplayName, webhook.URL, webhook.Method, webhook.Headers, events,
		webhook.IsEnabled, webhook.Secret, webhook.RetryCount, webhook.TimeoutMs,
		webhook.Organization, webhook.ContentType, webhook.CustomBody, webhook.CustomHeaders,
		webhook.CertID, webhook.IsError, webhook.ErrorText,
		tokenFields, objectFields, webhook.IsUserExtended,
		webhook.SingleOrgOnly, webhook.MaxRetries, webhook.RetryInterval, webhook.UseExponentialBackoff,
		webhook.UpdatedAt, webhook.ID, webhook.Revision,
	)
	n, err := rowsAffected(res, err)
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("webhook revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevision, webhook.ID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("webhook not found")
		}
		return err
	}
	webhook.Revision = newRevision
	return nil
}

// Delete 删除 webhook。
func (r Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteWebhook, id)
	return err
}

// ListByOrganizationAndEvent 按组织 + 事件类型查询已启用的 webhook。
// PG 版 events @> $2::jsonb（JSONB containment）→ MySQL 版 JSON_CONTAINS(events, ?)。
func (r Repository) ListByOrganizationAndEvent(ctx context.Context, organizationID, event string) ([]*domain.Webhook, error) {
	eventJSON, err := json.Marshal([]string{event})
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, queryListWebhooksByOrgAndEvent, organizationID, eventJSON)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.Webhook, 0)
	for rows.Next() {
		row, err := scanWebhookRow(rows)
		if err != nil {
			return nil, err
		}
		webhook, err := toWebhook(row)
		if err != nil {
			return nil, err
		}
		items = append(items, webhook)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return items, nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
