package models

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/models"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 models 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

type scannable interface {
	Scan(dest ...any) error
}

func scanModelRow(s scannable) (modelRow, error) {
	var row modelRow
	err := s.Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Description,
		&row.ModelText, &row.Owner, &row.IsEnabled,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	return row, err
}

// GetByID 按主键取 model（系统路径，不带请求租户）。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Model, error) {
	return r.getByIDQuery(ctx, queryGetModelByID, id)
}

// GetByIDForOrganization 按主键取 model（organization_id 谓词防止跨租户读取）。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Model, error) {
	return r.getByIDQuery(ctx, queryGetModelByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Model, error) {
	row, err := scanModelRow(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("model not found")
		}
		return nil, err
	}
	return &domain.Model{
		ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
		DisplayName: row.DisplayName, Description: row.Description,
		ModelText: row.ModelText, Owner: row.Owner, IsEnabled: row.IsEnabled,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
	}, nil
}

// List 分页列出 models。
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Model, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	var isEnabled any
	if query.IsEnabled != nil {
		isEnabled = *query.IsEnabled
	}

	rows, err := r.db.QueryContext(ctx, queryListModels,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Model, 0)
	for rows.Next() {
		row, err := scanModelRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, &domain.Model{
			ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
			DisplayName: row.DisplayName, Description: row.Description,
			ModelText: row.ModelText, Owner: row.Owner, IsEnabled: row.IsEnabled,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountModels,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create 插入 model。
func (r Repository) Create(ctx context.Context, m *domain.Model) error {
	_, err := r.db.ExecContext(ctx, queryInsertModel,
		m.ID, m.OrganizationID, m.Name, m.DisplayName, m.Description,
		m.ModelText, m.Owner, m.IsEnabled, m.CreatedAt, m.UpdatedAt, m.Revision)
	return err
}

// Update 乐观锁更新。UPDATE + SELECT revision；两处都带实体 organization_id 谓词。
func (r Repository) Update(ctx context.Context, m *domain.Model) error {
	res, err := r.db.ExecContext(ctx, queryUpdateModel,
		m.DisplayName, m.Description, m.ModelText, m.Owner, m.IsEnabled,
		m.UpdatedAt, m.ID, m.Revision, m.OrganizationID,
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
		return apperrors.NewConflict("model revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevision, m.ID, m.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("model not found")
		}
		return err
	}
	m.Revision = newRevision
	return nil
}

// Delete 删除 model（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteModel, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("model not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
