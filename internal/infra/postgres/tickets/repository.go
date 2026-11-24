package tickets

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Ticket, error) {
	var t domain.Ticket
	err := r.db.QueryRow(ctx, QueryGetByID, id, organizationID).Scan(&t.ID, &t.OrganizationID, &t.Name, &t.DisplayName, &t.Type, &t.Status, &t.Priority, &t.Assignee, &t.Creator, &t.Description, &t.Attachment, &t.ResolvedAt, &t.CreatedAt, &t.UpdatedAt, &t.Revision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("ticket not found")
		}
		return nil, err
	}
	return &t, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Ticket, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.Status, q.Assignee, q.Creator, q.Priority, q.Type, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Ticket, 0)
	for rows.Next() {
		var t domain.Ticket
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.Name, &t.DisplayName, &t.Type, &t.Status, &t.Priority, &t.Assignee, &t.Creator, &t.Description, &t.Attachment, &t.ResolvedAt, &t.CreatedAt, &t.UpdatedAt, &t.Revision); err != nil {
			return nil, 0, err
		}
		items = append(items, &t)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.Status, q.Assignee, q.Creator, q.Priority, q.Type).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, t *domain.Ticket) error {
	_, err := r.db.Exec(ctx, QueryInsert, t.ID, t.OrganizationID, t.Name, t.DisplayName, t.Type, t.Status, t.Priority, t.Assignee, t.Creator, t.Description, t.Attachment, t.ResolvedAt, t.CreatedAt, t.UpdatedAt, t.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, t *domain.Ticket) error {
	tag, err := r.db.Exec(ctx, QueryUpdate, t.DisplayName, t.Type, t.Status, t.Priority, t.Assignee, t.Description, t.Attachment, t.ResolvedAt, t.UpdatedAt, t.Revision, t.ID, t.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("ticket not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("ticket not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
