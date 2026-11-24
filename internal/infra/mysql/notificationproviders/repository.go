package notificationproviders

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 notificationproviders 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id 查 NotificationProvider。无租户谓词，仅供后台投递/告警等系统
// 调用；请求侧 by-ID 必须走 GetByIDForOrganization。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.NotificationProvider, error) {
	return r.getByID(ctx, queryGetNotificationProviderByID, id)
}

// GetByIDForOrganization 按 id + organization_id 查 NotificationProvider。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.NotificationProvider, error) {
	return r.getByID(ctx, queryGetNotificationProviderByIDForOrganization, id, organizationID)
}

func (r Repository) getByID(ctx context.Context, query string, args ...any) (*domain.NotificationProvider, error) {
	var row notificationProviderRow
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.WebhookURL, &row.AccessToken,
		&row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("notification provider not found")
		}
		return nil, err
	}
	return rowToDomain(row)
}

// List 分页列出 NotificationProvider。
// countArgs: org_id×2 + keyword×3 + is_enabled×2 = 7
// listArgs: 同上 + limit + offset = 9
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.NotificationProvider, int64, error) {
	var enabledArg interface{}
	if q.IsEnabled != nil {
		enabledArg = *q.IsEnabled
	}

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		enabledArg, enabledArg,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountNotificationProviders, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListNotificationProviders, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.NotificationProvider, 0)
	for rows.Next() {
		var row notificationProviderRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.WebhookURL, &row.AccessToken,
			&row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		p, err := rowToDomain(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 NotificationProvider。
func (r Repository) Create(ctx context.Context, provider *domain.NotificationProvider) error {
	config, err := mysqlshared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}
	accessToken, err := sealAccessToken(provider)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertNotificationProvider,
		provider.ID, provider.OrganizationID, provider.ID, provider.Name, provider.ProviderType,
		provider.WebhookURL, accessToken, config, provider.IsEnabled,
		provider.CreatedAt, provider.UpdatedAt, provider.Revision,
	)
	return err
}

// Update 更新 NotificationProvider（乐观锁）。
func (r Repository) Update(ctx context.Context, provider *domain.NotificationProvider) error {
	config, err := mysqlshared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}

	accessToken, err := sealAccessToken(provider)
	if err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx, queryUpdateNotificationProvider,
		provider.Name, provider.ProviderType, provider.WebhookURL, accessToken,
		config, provider.IsEnabled, provider.UpdatedAt,
		provider.ID, provider.Revision-1, provider.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("notification provider revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevisionNotificationProvider, provider.ID, provider.OrganizationID).Scan(&newRevision); err != nil {
		return err
	}
	provider.Revision = newRevision
	return nil
}

// Delete 删除 NotificationProvider（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteNotificationProvider, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("notification provider not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
