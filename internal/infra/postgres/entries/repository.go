package entries

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Entry, error) {
	e, err := scanEntry(r.db.QueryRow(ctx, QueryGetByID, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("entry not found")
		}
		return nil, err
	}
	return e, nil
}

// GetByIDForOrganization reads an entry only when it belongs to organizationID.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Entry, error) {
	e, err := scanEntry(r.db.QueryRow(ctx, QueryGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("entry not found")
		}
		return nil, err
	}
	return e, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Entry, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.Type, q.Application, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Entry, 0)
	for rows.Next() {
		var e domain.Entry
		if err := rows.Scan(&e.ID, &e.OrganizationID, &e.Name, &e.DisplayName, &e.Type, &e.Application, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &e)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.Type, q.Application).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, e *domain.Entry) error {
	_, err := r.db.Exec(ctx, QueryInsert, e.ID, e.OrganizationID, e.Name, e.DisplayName, e.Provider, e.Application, e.Type, e.ClientIP, e.UserAgent, e.Message, e.CreatedAt, e.UpdatedAt, e.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, e *domain.Entry) error {
	_, err := r.db.Exec(ctx, QueryUpdate, e.DisplayName, e.Message, e.UpdatedAt, e.Revision, e.ID, e.OrganizationID)
	return err
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("entry not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
