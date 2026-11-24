package resources

import (
	"context"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/resources"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Resource, error) {
	var row resourceRow
	var applicationID *string
	err := r.db.QueryRow(ctx, queryGetResourceByID, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &applicationID, &row.Name, &row.DisplayName,
		&row.Type, &row.Tag, &row.FilePath, &row.FileSize, &row.MimeType,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	row.ApplicationID = stringOrEmpty(applicationID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("resource not found")
		}
		return nil, err
	}

	return toResource(row), nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Resource, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListResources,
		query.OrganizationID, query.ApplicationID, query.Keyword, query.Type, query.Tag,
		shared.ClampPageSize(query.PageSize), offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Resource, 0)
	for rows.Next() {
		var row resourceRow
		var applicationID *string
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &applicationID, &row.Name, &row.DisplayName,
			&row.Type, &row.Tag, &row.FilePath, &row.FileSize, &row.MimeType,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		row.ApplicationID = stringOrEmpty(applicationID)
		items = append(items, toResource(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountResources,
		query.OrganizationID, query.ApplicationID, query.Keyword, query.Type, query.Tag,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, resource *domain.Resource) error {
	_, err := r.db.Exec(ctx, queryInsertResource,
		resource.ID, resource.OrganizationID, nullableUUID(resource.ApplicationID), resource.Name, resource.DisplayName,
		resource.Type, resource.Tag, resource.FilePath, resource.FileSize, resource.MimeType,
		resource.CreatedAt, resource.UpdatedAt, resource.Revision,
	)
	return err
}

func (r Repository) Update(ctx context.Context, resource *domain.Resource) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryUpdateResource,
		resource.DisplayName, resource.Type, resource.Tag, resource.FilePath,
		// App 层已 Revision++，这里要传自增前的值做乐观锁守卫。
		resource.FileSize, resource.MimeType, resource.UpdatedAt, resource.Revision-1, resource.ID,
		resource.OrganizationID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			// 实体已在 app 层按租户读取：0 行语义仍是 revision 冲突。
			return apperrors.NewConflict("resource revision conflict")
		}
		return err
	}
	resource.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteResource, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("resource not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// nullableUUID 把空字符串转成 NULL：application_id 是 uuid 列，直接写 "" 会触发
// SQLSTATE 22P02（invalid input syntax for type uuid）。
func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
