package captchaproviders

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 captchaproviders 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id + organization_id 查 CaptchaProvider。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.CaptchaProvider, error) {
	var row captchaProviderRow
	err := r.db.QueryRowContext(ctx, queryGetCaptchaProviderByID, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.SiteKey, &row.SecretKey,
		&row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("captcha provider not found")
		}
		return nil, err
	}
	return rowToDomain(row)
}

// List 分页列出 CaptchaProvider。
// countArgs: org_id×2 + keyword×3 + is_enabled×2 = 7
// listArgs: 同上 + limit + offset = 9
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.CaptchaProvider, int64, error) {
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
	if err := r.db.QueryRowContext(ctx, queryCountCaptchaProviders, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListCaptchaProviders, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.CaptchaProvider, 0)
	for rows.Next() {
		var row captchaProviderRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.SiteKey, &row.SecretKey,
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

// Create 插入 CaptchaProvider。
func (r Repository) Create(ctx context.Context, provider *domain.CaptchaProvider) error {
	config, err := mysqlshared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}
	secret, err := captchaSecretForWrite(provider)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertCaptchaProvider,
		provider.ID, provider.OrganizationID, provider.Name, provider.ProviderType,
		provider.SiteKey, secret, config, provider.IsEnabled,
		provider.CreatedAt, provider.UpdatedAt, provider.Revision,
	)
	return err
}

// Update 更新 CaptchaProvider（乐观锁，MySQL 版拆成 UPDATE + SELECT 两步）。
// 实体 OrganizationID 同时作为谓词；app 层已 Revision++，WHERE 用 Revision-1。
// 0 affected rows 表示 revision 不匹配（MySQL 默认只统计实际变更行；跨租户在
// app 层 scoped GetByID 已返回 NotFound，正常不会走到这里）。
func (r Repository) Update(ctx context.Context, provider *domain.CaptchaProvider) error {
	config, err := mysqlshared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}
	secret, err := captchaSecretForWrite(provider)
	if err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx, queryUpdateCaptchaProvider,
		provider.Name, provider.ProviderType, provider.SiteKey, secret,
		config, provider.IsEnabled, provider.UpdatedAt, provider.ID, provider.Revision-1, provider.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("captcha provider revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevisionCaptchaProvider, provider.ID, provider.OrganizationID).Scan(&newRevision); err != nil {
		return err
	}
	provider.Revision = newRevision
	return nil
}

// Delete 删除 CaptchaProvider（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteCaptchaProvider, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("captcha provider not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
