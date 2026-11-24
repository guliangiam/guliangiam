package transactions

import (
	"context"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return r.getByIDWithQuery(ctx, QueryGetByID, id)
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Transaction, error) {
	return r.getByIDWithQuery(ctx, QueryGetByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDWithQuery(ctx context.Context, query string, args ...any) (*domain.Transaction, error) {
	t, err := scanTransaction(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("transaction not found")
		}
		return nil, err
	}
	return t, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Transaction, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.UserID, q.OrderID, q.Status, q.TransactionType, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Transaction, 0)
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, t)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.UserID, q.OrderID, q.Status, q.TransactionType).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, t *domain.Transaction) error {
	var metadata json.RawMessage
	if t.Metadata != nil {
		metadata = t.Metadata
	} else {
		metadata = json.RawMessage(`{}`)
	}
	_, err := r.db.Exec(ctx, QueryInsert, t.ID, t.OrganizationID, nilIfEmpty(t.UserID), nilIfEmpty(t.OrderID), nilIfEmpty(t.PaymentID), t.TransactionType, t.Amount, t.Currency, t.Status, t.ProviderType, nilIfEmpty(t.ProviderID), metadata, t.CreatedAt, t.UpdatedAt, t.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, t *domain.Transaction) error {
	var metadata json.RawMessage
	if t.Metadata != nil {
		metadata = t.Metadata
	} else {
		metadata = json.RawMessage(`{}`)
	}
	tag, err := r.db.Exec(ctx, QueryUpdate, t.Status, metadata, t.UpdatedAt, t.Revision, t.ID, t.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("transaction not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("transaction not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
