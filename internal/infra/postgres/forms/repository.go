package forms

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/forms"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// pgerrorConstraints is the whitelist of constraint identifiers this repository
// may surface in error metadata; anything else stays in the server log.
var pgerrorConstraints = []string{
	"uq_forms_org_name",
	"fk_forms_organization_id_organizations",
}

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Form, error) {
	f, err := scanForm(r.db.QueryRow(ctx, QueryGetByID, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("form not found")
		}
		return nil, err
	}
	return f, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Form, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.IsEnabled, q.Type, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Form, 0)
	for rows.Next() {
		f, err := scanForm(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, f)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.IsEnabled, q.Type).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, f *domain.Form) error {
	_, err := r.db.Exec(ctx, QueryInsert, f.ID, f.OrganizationID, f.Name, f.DisplayName, f.Description, f.Type, f.Properties, f.IsEnabled, f.CreatedAt, f.UpdatedAt, f.Revision)
	return pgerror.Map(err, pgerrorConstraints...)
}

func (r Repository) Update(ctx context.Context, f *domain.Form) error {
	tag, err := r.db.Exec(ctx, QueryUpdate, f.DisplayName, f.Description, f.Type, f.Properties, f.IsEnabled, f.UpdatedAt, f.Revision, f.ID, f.OrganizationID)
	if err != nil {
		return pgerror.Map(err, pgerrorConstraints...)
	}
	if tag.RowsAffected() == 0 {
		// The app layer read the row scoped first; zero rows means it is gone.
		return apperrors.NewNotFound("form not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return pgerror.Map(err, pgerrorConstraints...)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("form not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
