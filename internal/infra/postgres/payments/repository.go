package payments

import (
	"context"
	"encoding/json"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	p, err := scanPayment(r.db.QueryRow(ctx, QueryGetByID, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("payment not found")
		}
		return nil, err
	}
	return p, nil
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Payment, error) {
	p, err := scanPayment(r.db.QueryRow(ctx, QueryGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("payment not found")
		}
		return nil, err
	}
	return p, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Payment, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.User, q.State, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Payment, 0)
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.User, q.State).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, p *domain.Payment) error {
	products, _ := json.Marshal(p.Products)
	var idempotencyKey interface{}
	if p.IdempotencyKey != "" {
		idempotencyKey = p.IdempotencyKey
	}
	_, err := r.db.Exec(ctx, QueryInsert, p.ID, p.OrganizationID, p.Name, p.DisplayName, p.Provider, p.Type, products, p.ProductName, p.ProductDisplayName, p.Detail, p.Currency, p.Price, nullableUUID(p.User), p.PersonName, p.PersonIdCard, p.PersonEmail, p.PersonPhone, p.InvoiceType, p.InvoiceTitle, p.InvoiceTaxId, p.InvoiceRemark, p.InvoiceUrl, nullableUUID(p.Order), p.OutOrderId, p.PayUrl, p.SuccessUrl, p.State, p.Message, idempotencyKey, p.CreatedAt, p.UpdatedAt, p.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, p *domain.Payment) error {
	var idempotencyKey interface{}
	if p.IdempotencyKey != "" {
		idempotencyKey = p.IdempotencyKey
	}
	tag, err := r.db.Exec(ctx, QueryUpdate, p.DisplayName, p.Provider, p.Type, p.State, p.Message, p.PayUrl, p.SuccessUrl, p.OutOrderId, p.InvoiceUrl, idempotencyKey, p.UpdatedAt, p.Revision, p.ID, p.OrganizationID, p.ChannelOrderNo, p.AttemptCount, p.LastErrorCode, p.LastErrorMessage)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Callers read the row scoped first; zero rows means it is gone.
		return apperrors.NewNotFound("payment not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("payment not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

// nullableUUID 把空字符串转成 NULL：user_id / order_id 是 uuid 列，直接写 ""
// 会触发 SQLSTATE 22P02（invalid input syntax for type uuid）。
func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
