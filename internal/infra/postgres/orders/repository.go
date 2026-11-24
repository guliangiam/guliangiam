package orders

import (
	"context"
	"encoding/json"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	return r.getByIDWithQuery(ctx, QueryGetByID, id)
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Order, error) {
	return r.getByIDWithQuery(ctx, QueryGetByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDWithQuery(ctx context.Context, query string, args ...any) (*domain.Order, error) {
	o, err := scanOrder(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("order not found")
		}
		return nil, err
	}
	return o, nil
}

func (r Repository) GetByTradeNo(ctx context.Context, tradeNo string) (*domain.Order, error) {
	o, err := scanOrder(r.db.QueryRow(ctx, QueryGetByTradeNo, tradeNo))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("order not found by trade_no")
		}
		return nil, err
	}
	return o, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Order, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.User, q.State, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, o)
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

func (r Repository) Create(ctx context.Context, o *domain.Order) error {
	products, _ := json.Marshal(o.Products)
	productInfos, _ := json.Marshal(o.ProductInfos)
	var idempotencyKey interface{}
	if o.IdempotencyKey != "" {
		idempotencyKey = o.IdempotencyKey
	}
	_, err := r.db.Exec(ctx, QueryInsert, o.ID, o.OrganizationID, o.Name, o.DisplayName, nullableUUID(o.User), products, productInfos, o.Payment, o.Price, o.Currency, o.State, o.Message, o.PaymentURL, o.TradeNo, o.CouponName, o.CouponDiscount, idempotencyKey, o.CreatedAt, o.UpdatedAt, o.PaidAt, o.Revision)
	// 22P02（非法 user_id/products UUID）等必须映射为 4xx，不能泄漏原始 PgError 成 500。
	return pgerror.Map(err, "uq_orders_org_name")
}

func (r Repository) Update(ctx context.Context, o *domain.Order) error {
	var idempotencyKey interface{}
	if o.IdempotencyKey != "" {
		idempotencyKey = o.IdempotencyKey
	}
	tag, err := r.db.Exec(ctx, QueryUpdate, o.DisplayName, nullableUUID(o.User), o.Payment, o.Price, o.Currency, o.State, o.Message, o.PaymentURL, o.TradeNo, o.CouponName, o.CouponDiscount, idempotencyKey, o.UpdatedAt, o.PaidAt, o.Revision, o.ID, o.OrganizationID)
	if err != nil {
		return pgerror.Map(err, "uq_orders_org_name")
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("order not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("order not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

// nullableUUID 把空字符串转成 NULL：user_id 是 uuid 列，直接写 "" 会触发
// SQLSTATE 22P02（invalid input syntax for type uuid）。
func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
