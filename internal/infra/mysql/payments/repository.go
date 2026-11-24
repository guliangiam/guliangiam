package payments

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 payments 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id 查 Payment。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	p, err := scanPaymentRow(r.db.QueryRowContext(ctx, queryGetPaymentByID, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("payment not found")
		}
		return nil, err
	}
	return p, nil
}

// GetByIDForOrganization only returns the payment when it belongs to the
// trusted tenant; MySQL has no RLS, so the predicate is the isolation boundary.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Payment, error) {
	p, err := scanPaymentRow(r.db.QueryRowContext(ctx, queryGetPaymentByIDForOrganization, id, organizationID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("payment not found")
		}
		return nil, err
	}
	return p, nil
}

// List 分页列出 Payment。MySQL 版需要手动展开参数重复占位符。
// countArgs: org_id×2 + keyword×3 + user_id×2 + state×2 = 9
// listArgs: 同上 + limit + offset = 11
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Payment, int64, error) {
	var stateArg interface{}
	if q.State != nil {
		stateArg = *q.State
	} // 否则传 nil

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.User, q.User,
		stateArg, stateArg,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountPayments, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListPayments, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Payment, 0)
	for rows.Next() {
		p, err := scanPaymentRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 Payment，products 用 JSON 字符串，idempotency_key nullable → sql.NullString。
func (r Repository) Create(ctx context.Context, p *domain.Payment) error {
	productsJSON, err := json.Marshal(p.Products)
	if err != nil {
		return err
	}
	var idempotencyKey sql.NullString
	if p.IdempotencyKey != "" {
		idempotencyKey = sql.NullString{String: p.IdempotencyKey, Valid: true}
	}
	_, err = r.db.ExecContext(ctx, queryInsertPayment,
		p.ID, p.OrganizationID, p.Name, p.DisplayName, p.Provider, p.Type,
		string(productsJSON), p.ProductName, p.ProductDisplayName, p.Detail,
		p.Currency, p.Price, p.User, p.PersonName, p.PersonIdCard,
		p.PersonEmail, p.PersonPhone, p.InvoiceType, p.InvoiceTitle,
		p.InvoiceTaxId, p.InvoiceRemark, p.InvoiceUrl, p.Order,
		p.OutOrderId, p.PayUrl, p.SuccessUrl, p.State, p.Message,
		idempotencyKey, p.CreatedAt, p.UpdatedAt, p.Revision,
	)
	return err
}

// Update 更新 Payment。PG 版没有 RETURNING，MySQL 直接 ExecContext。
func (r Repository) Update(ctx context.Context, p *domain.Payment) error {
	var idempotencyKey sql.NullString
	if p.IdempotencyKey != "" {
		idempotencyKey = sql.NullString{String: p.IdempotencyKey, Valid: true}
	}
	_, err := r.db.ExecContext(ctx, queryUpdatePayment,
		p.DisplayName, p.Provider, p.Type, p.State, p.Message,
		p.PayUrl, p.SuccessUrl, p.OutOrderId, p.InvoiceUrl,
		idempotencyKey, p.UpdatedAt, p.Revision, p.ID, p.OrganizationID,
	)
	return err
}

// Delete 删除 Payment；organization_id 谓词让跨租户 id 与不存在一样返回 NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeletePayment, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("payment not found")
	}
	return nil
}

// scannable 是 sql.Row 和 sql.Rows 的公共接口。
type scannable interface {
	Scan(dest ...any) error
}

// scanPaymentRow 扫描一行 Payment，处理 products JSON 和 idempotency_key nullable。
func scanPaymentRow(sc scannable) (*domain.Payment, error) {
	var p domain.Payment
	var productsStr string
	var idempotencyKey sql.NullString
	err := sc.Scan(
		&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.Provider, &p.Type,
		&productsStr, &p.ProductName, &p.ProductDisplayName, &p.Detail,
		&p.Currency, &p.Price, &p.User, &p.PersonName, &p.PersonIdCard,
		&p.PersonEmail, &p.PersonPhone, &p.InvoiceType, &p.InvoiceTitle,
		&p.InvoiceTaxId, &p.InvoiceRemark, &p.InvoiceUrl, &p.Order,
		&p.OutOrderId, &p.PayUrl, &p.SuccessUrl, &p.State, &p.Message,
		&idempotencyKey, &p.CreatedAt, &p.UpdatedAt, &p.Revision,
	)
	if err != nil {
		return nil, err
	}
	if idempotencyKey.Valid {
		p.IdempotencyKey = idempotencyKey.String
	}
	if productsStr != "" && productsStr != jsonNullLiteral {
		if err := json.Unmarshal([]byte(productsStr), &p.Products); err != nil {
			return nil, apperrors.NewInternal("unmarshal products")
		}
	}
	if p.Products == nil {
		p.Products = []string{}
	}
	return &p, nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
