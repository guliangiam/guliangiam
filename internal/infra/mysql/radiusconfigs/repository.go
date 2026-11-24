package radiusconfigs

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 radiusconfigs 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id 查 RadiusConfig（organization_id 谓词防止跨租户读取）。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.RadiusConfig, error) {
	var row radiusConfigRow
	err := r.db.QueryRowContext(ctx, queryGetRadiusConfigByID, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.Host, &row.Port, &row.SharedSecret,
		&row.AuthProtocol, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("radius config not found")
		}
		return nil, err
	}
	return rowToDomain(row)
}

// List 分页列出 RadiusConfig。
// countArgs: org_id×2 + is_enabled×2 + auth_protocol×2 = 6
// listArgs: 同上 + limit + offset = 8
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.RadiusConfig, int64, error) {
	var enabledArg interface{}
	if q.IsEnabled != nil {
		enabledArg = *q.IsEnabled
	}

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		enabledArg, enabledArg,
		q.AuthProtocol, q.AuthProtocol,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountRadiusConfigs, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListRadiusConfigs, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.RadiusConfig, 0)
	for rows.Next() {
		var row radiusConfigRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.Host, &row.Port, &row.SharedSecret,
			&row.AuthProtocol, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		config, err := rowToDomain(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, config)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 RadiusConfig。
func (r Repository) Create(ctx context.Context, config *domain.RadiusConfig) error {
	secret, err := radiusSecretForWrite(config)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertRadiusConfig,
		config.ID, config.OrganizationID, config.Name, config.Host, config.Port, secret,
		config.AuthProtocol, config.IsEnabled, config.CreatedAt, config.UpdatedAt, config.Revision,
	)
	return err
}

// Update 更新 RadiusConfig（乐观锁）。
func (r Repository) Update(ctx context.Context, config *domain.RadiusConfig) error {
	secret, err := radiusSecretForWrite(config)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, queryUpdateRadiusConfig,
		config.Name, config.Host, config.Port, secret, config.AuthProtocol,
		config.IsEnabled, config.UpdatedAt, config.ID, config.Revision-1, config.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// 实体已在 app 层按租户读取：0 affected 只代表 revision 冲突。
		return apperrors.NewConflict("radius config revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevisionRadiusConfig, config.ID, config.OrganizationID).Scan(&newRevision); err != nil {
		return err
	}
	config.Revision = newRevision
	return nil
}

// Delete 删除 RadiusConfig（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteRadiusConfig, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("radius config not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
