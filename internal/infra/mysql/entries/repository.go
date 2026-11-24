package entries

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 entries 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id 查 Entry。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Entry, error) {
	var e domain.Entry
	err := r.db.QueryRowContext(ctx, queryGetEntryByID, id).Scan(
		&e.ID, &e.OrganizationID, &e.Name, &e.DisplayName, &e.Provider, &e.Application,
		&e.Type, &e.ClientIP, &e.UserAgent, &e.Message, &e.CreatedAt, &e.UpdatedAt, &e.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("entry not found")
		}
		return nil, err
	}
	return &e, nil
}

// GetByIDForOrganization 按 id + organization_id 查 Entry；跨租户按不存在处理。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Entry, error) {
	var e domain.Entry
	err := r.db.QueryRowContext(ctx, queryGetEntryByIDForOrganization, id, organizationID).Scan(
		&e.ID, &e.OrganizationID, &e.Name, &e.DisplayName, &e.Provider, &e.Application,
		&e.Type, &e.ClientIP, &e.UserAgent, &e.Message, &e.CreatedAt, &e.UpdatedAt, &e.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("entry not found")
		}
		return nil, err
	}
	return &e, nil
}

// List 分页列出 Entry。MySQL 版需要手动展开参数重复占位符。
// countArgs: org_id×2 + keyword×4 + type×2 + application×2 = 10
// listArgs: 同上 + limit + offset = 12
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Entry, int64, error) {
	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword, q.Keyword,
		q.Type, q.Type,
		q.Application, q.Application,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountEntries, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListEntries, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Entry, 0)
	for rows.Next() {
		var e domain.Entry
		if err := rows.Scan(
			&e.ID, &e.OrganizationID, &e.Name, &e.DisplayName, &e.Type, &e.Application, &e.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, &e)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 Entry。
func (r Repository) Create(ctx context.Context, e *domain.Entry) error {
	_, err := r.db.ExecContext(ctx, queryInsertEntry,
		e.ID, e.OrganizationID, e.Name, e.DisplayName, e.Provider, e.Application,
		e.Type, e.ClientIP, e.UserAgent, e.Message, e.CreatedAt, e.UpdatedAt, e.Revision,
	)
	return err
}

// Update 更新 Entry。PG 版没有 RETURNING，MySQL 直接 ExecContext。
func (r Repository) Update(ctx context.Context, e *domain.Entry) error {
	_, err := r.db.ExecContext(ctx, queryUpdateEntry,
		e.DisplayName, e.Message, e.UpdatedAt, e.Revision, e.ID, e.OrganizationID,
	)
	return err
}

// Delete 按 id + organization_id 删除 Entry；跨租户按不存在处理。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteEntry, id, organizationID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("entry not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
