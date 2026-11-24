package enforcers

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 enforcers 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

type scannable interface {
	Scan(dest ...any) error
}

func scanEnforcerRow(s scannable) (enforcerRow, error) {
	var row enforcerRow
	err := s.Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Description,
		&row.ModelID, &row.AdapterID, &row.IsEnabled, &row.Owner,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	return row, err
}

// GetByID 按主键取 enforcer（系统/后台任务路径，不带请求租户）。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Enforcer, error) {
	return r.getByIDQuery(ctx, queryGetEnforcerByID, id)
}

// GetByIDForOrganization 只在 enforcer 属于该组织时返回；跨租户与不存在统一 NotFound。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Enforcer, error) {
	return r.getByIDQuery(ctx, queryGetEnforcerByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Enforcer, error) {
	row, err := scanEnforcerRow(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("enforcer not found")
		}
		return nil, err
	}
	return rowToDomain(row), nil
}

// List 分页列出 enforcers。
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Enforcer, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	var isEnabled any
	if query.IsEnabled != nil {
		isEnabled = *query.IsEnabled
	}

	rows, err := r.db.QueryContext(ctx, queryListEnforcers,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Enforcer, 0)
	for rows.Next() {
		row, err := scanEnforcerRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, rowToDomain(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountEnforcers,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create 插入 enforcer。
func (r Repository) Create(ctx context.Context, e *domain.Enforcer) error {
	_, err := r.db.ExecContext(ctx, queryInsertEnforcer,
		e.ID, e.OrganizationID, e.Name, e.DisplayName, e.Description,
		e.ModelID, e.AdapterID, e.IsEnabled, e.Owner,
		e.CreatedAt, e.UpdatedAt, e.Revision)
	return err
}

// Update 乐观锁更新。UPDATE + SELECT revision；两处都带实体 organization_id 谓词。
func (r Repository) Update(ctx context.Context, e *domain.Enforcer) error {
	res, err := r.db.ExecContext(ctx, queryUpdateEnforcer,
		e.DisplayName, e.Description, e.ModelID, e.AdapterID, e.IsEnabled,
		e.Owner, e.UpdatedAt, e.ID, e.Revision, e.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("enforcer revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevision, e.ID, e.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("enforcer not found")
		}
		return err
	}
	e.Revision = newRevision
	return nil
}

// Delete 只在 enforcer 属于该组织时删除；0 行受影响统一 NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteEnforcer, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("enforcer not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
