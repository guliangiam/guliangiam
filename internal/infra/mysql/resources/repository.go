package resources

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/resources"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 resources 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id + organization_id 查 Resource（跨租户 id 与不存在同为 NotFound）。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Resource, error) {
	var res domain.Resource
	err := r.db.QueryRowContext(ctx, queryGetResourceByID, id, organizationID).Scan(
		&res.ID, &res.OrganizationID, &res.ApplicationID, &res.Name, &res.DisplayName,
		&res.Type, &res.Tag, &res.FilePath, &res.FileSize, &res.MimeType,
		&res.CreatedAt, &res.UpdatedAt, &res.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("resource not found")
		}
		return nil, err
	}
	return &res, nil
}

// List 分页列出 Resource。
// countArgs: org_id×2 + app_id×2 + keyword×3 + type×2 + tag×2 = 11
// listArgs: 同上 + limit + offset = 13
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Resource, int64, error) {
	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.ApplicationID, q.ApplicationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.Type, q.Type,
		q.Tag, q.Tag,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountResources, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListResources, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Resource, 0)
	for rows.Next() {
		var res domain.Resource
		if err := rows.Scan(
			&res.ID, &res.OrganizationID, &res.ApplicationID, &res.Name, &res.DisplayName,
			&res.Type, &res.Tag, &res.FilePath, &res.FileSize, &res.MimeType,
			&res.CreatedAt, &res.UpdatedAt, &res.Revision,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, &res)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 Resource。
func (r Repository) Create(ctx context.Context, res *domain.Resource) error {
	_, err := r.db.ExecContext(ctx, queryInsertResource,
		res.ID, res.OrganizationID, res.ApplicationID, res.Name, res.DisplayName,
		res.Type, res.Tag, res.FilePath, res.FileSize, res.MimeType,
		res.CreatedAt, res.UpdatedAt, res.Revision,
	)
	return err
}

// Update 乐观锁更新 Resource（organization_id 取自实体）。先 UPDATE，再 SELECT 新 revision。
func (r Repository) Update(ctx context.Context, res *domain.Resource) error {
	exec, err := r.db.ExecContext(ctx, queryUpdateResource,
		res.DisplayName, res.Type, res.Tag, res.FilePath,
		res.FileSize, res.MimeType, res.UpdatedAt, res.ID, res.Revision, res.OrganizationID,
	)
	if err != nil {
		return err
	}
	affected, err := exec.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// 实体已在 app 层按租户读取：0 行语义仍是 revision 冲突。
		return apperrors.NewConflict("resource revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectResourceRevision, res.ID, res.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("resource not found")
		}
		return err
	}
	res.Revision = newRevision
	return nil
}

// Delete 只在 Resource 属于该组织时删除；0 行受影响 → NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteResource, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("resource not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
