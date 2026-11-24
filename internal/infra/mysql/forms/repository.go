package forms

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/forms"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 forms 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id 查 Form（organization_id 谓词防止跨租户读取）。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Form, error) {
	var f domain.Form
	err := r.db.QueryRowContext(ctx, queryGetFormByID, id, organizationID).Scan(
		&f.ID, &f.OrganizationID, &f.Name, &f.DisplayName, &f.Description, &f.Type,
		&f.Properties, &f.IsEnabled, &f.CreatedAt, &f.UpdatedAt, &f.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("form not found")
		}
		return nil, err
	}
	return &f, nil
}

// List 分页列出 Form。MySQL 版需要手动展开参数重复占位符。
// countArgs: org_id×2 + keyword×3 + is_enabled×2 + type×2 = 9
// listArgs: 同上 + limit + offset = 11
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Form, int64, error) {
	var enabledArg interface{}
	if q.IsEnabled != nil {
		enabledArg = *q.IsEnabled
	} // 否则传 nil

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		enabledArg, enabledArg,
		q.Type, q.Type,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountForms, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListForms, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Form, 0)
	for rows.Next() {
		var f domain.Form
		if err := rows.Scan(
			&f.ID, &f.OrganizationID, &f.Name, &f.DisplayName, &f.Description, &f.Type,
			&f.Properties, &f.IsEnabled, &f.CreatedAt, &f.UpdatedAt, &f.Revision,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, &f)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 Form。
func (r Repository) Create(ctx context.Context, f *domain.Form) error {
	_, err := r.db.ExecContext(ctx, queryInsertForm,
		f.ID, f.OrganizationID, f.Name, f.DisplayName, f.Description, f.Type,
		f.Properties, f.IsEnabled, f.CreatedAt, f.UpdatedAt, f.Revision,
	)
	return err
}

// Update 更新 Form。PG 版没有 RETURNING，MySQL 直接 ExecContext。
func (r Repository) Update(ctx context.Context, f *domain.Form) error {
	res, err := r.db.ExecContext(ctx, queryUpdateForm,
		f.DisplayName, f.Description, f.Type, f.Properties, f.IsEnabled,
		f.UpdatedAt, f.Revision, f.ID, f.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// 实体已在 app 层按租户读取：0 affected 表示行已被并发删除。
		return apperrors.NewNotFound("form not found")
	}
	return nil
}

// Delete 删除 Form（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteForm, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("form not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
