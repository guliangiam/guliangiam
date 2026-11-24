package faceidproviders

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 faceidproviders 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id 查 FaceIdProvider（organization_id 谓词防止跨租户读取）。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.FaceIdProvider, error) {
	var row faceIdProviderRow
	err := r.db.QueryRowContext(ctx, queryGetFaceIdProviderByID, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.AccessKeyID, &row.AccessKeySecret,
		&row.Endpoint, &row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("face id provider not found")
		}
		return nil, err
	}
	p, err := rowToDomain(row)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode face id provider")
	}
	return p, nil
}

// List 分页列出 FaceIdProvider。
// countArgs: org_id×2 + is_enabled×2 = 4
// listArgs: 同上 + limit + offset = 6
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.FaceIdProvider, int64, error) {
	var enabledArg interface{}
	if q.IsEnabled != nil {
		enabledArg = *q.IsEnabled
	}

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		enabledArg, enabledArg,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountFaceIdProviders, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListFaceIdProviders, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.FaceIdProvider, 0)
	for rows.Next() {
		var row faceIdProviderRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.AccessKeyID, &row.AccessKeySecret,
			&row.Endpoint, &row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		p, err := rowToDomain(row)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "decode face id provider")
		}
		items = append(items, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 FaceIdProvider。
func (r Repository) Create(ctx context.Context, provider *domain.FaceIdProvider) error {
	config, err := mysqlshared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}
	accessKeySecret, err := sealAccessKeySecret(provider)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertFaceIdProvider,
		provider.ID, provider.OrganizationID, provider.Name, provider.ProviderType,
		provider.AccessKeyID, accessKeySecret, provider.Endpoint, config,
		provider.IsEnabled, provider.CreatedAt, provider.UpdatedAt, provider.Revision,
	)
	return err
}

// Update 更新 FaceIdProvider（乐观锁，MySQL 版拆成 UPDATE + SELECT 两步）。
func (r Repository) Update(ctx context.Context, provider *domain.FaceIdProvider) error {
	config, err := mysqlshared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}
	accessKeySecret, err := sealAccessKeySecret(provider)
	if err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx, queryUpdateFaceIdProvider,
		provider.Name, provider.ProviderType, provider.AccessKeyID, accessKeySecret,
		provider.Endpoint, config, provider.IsEnabled, provider.UpdatedAt,
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
		// 实体已在 app 层按租户读取：0 affected 只代表 revision 冲突。
		return apperrors.NewConflict("face id provider revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevisionFaceIdProvider, provider.ID, provider.OrganizationID).Scan(&newRevision); err != nil {
		return err
	}
	provider.Revision = newRevision
	return nil
}

// Delete 删除 FaceIdProvider（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteFaceIdProvider, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("face id provider not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
