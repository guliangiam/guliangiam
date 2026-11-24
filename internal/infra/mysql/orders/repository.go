package orders

import (
	"context"
	"database/sql"
	"encoding/json"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
)

// Repository 是 MySQL 8.0+ 实现的 orders 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// rowsAffected 从 sql.Result 安全提取影响行数。
func rowsAffected(r sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ── domain.Repository 实现 ────────────────────────────────────────

// GetByID 按主键取订单（系统作用域；HTTP 管理路由必须用 GetByIDForOrganization）。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	return r.getByIDWithQuery(ctx, queryGetOrderByID, id)
}

// GetByIDForOrganization 只在订单属于该组织时返回；跨租户与不存在统一 NotFound。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Order, error) {
	return r.getByIDWithQuery(ctx, queryGetOrderByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDWithQuery(ctx context.Context, query string, args ...any) (*domain.Order, error) {
	o, err := scanOrder(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("order not found")
		}
		return nil, err
	}
	return o, nil
}

// GetByTradeNo 按 trade_no 取订单。
func (r Repository) GetByTradeNo(ctx context.Context, tradeNo string) (*domain.Order, error) {
	o, err := scanOrder(r.db.QueryRowContext(ctx, queryGetOrderByTradeNo, tradeNo))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("order not found by trade_no")
		}
		return nil, err
	}
	return o, nil
}

// List 分页列出订单。
// MySQL 版参数绑定（占位符重复）：
//
//	org_id×2, keyword×3, user×2, state×2, limit, offset = 11
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Order, int64, error) {
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	limit := mysqlshared.ClampPageSize(q.PageSize)

	rows, err := r.db.QueryContext(ctx, queryListOrders,
		q.OrganizationID, q.OrganizationID, // org_id × 2
		q.Keyword, q.Keyword, q.Keyword, // keyword × 3
		q.User, q.User, // user × 2
		q.State, q.State, // state × 2
		limit, offset,
	)
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

	// Count 参数：org_id×2, keyword×3, user×2, state×2 = 9
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountOrders,
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.User, q.User,
		q.State, q.State,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Create 插入订单。
// 参数顺序：与 INSERT 列严格对齐，21 个 ?。
func (r Repository) Create(ctx context.Context, o *domain.Order) error {
	products, err := json.Marshal(o.Products)
	if err != nil {
		return err
	}
	productInfos, err := json.Marshal(o.ProductInfos)
	if err != nil {
		return err
	}

	var idempotencyKey interface{}
	if o.IdempotencyKey != "" {
		idempotencyKey = o.IdempotencyKey
	}

	_, err = r.db.ExecContext(ctx, queryInsertOrder,
		o.ID, o.OrganizationID, o.Name, o.DisplayName, o.User,
		products, productInfos,
		o.Payment, o.Price, o.Currency, o.State, o.Message,
		o.PaymentURL, o.TradeNo, o.CouponName, o.CouponDiscount,
		idempotencyKey,
		o.CreatedAt, o.UpdatedAt, o.PaidAt, o.Revision,
	)
	return err
}

// Update 乐观锁更新订单（带租户谓词）。
// MySQL 版拆成两步（不带 RETURNING）：
//  1. UPDATE ... WHERE id = ? AND revision = ? AND organization_id = ? → 判断影响行数
//  2. SELECT revision FROM ... WHERE id = ? AND organization_id = ? → 回读新 revision
//
// 参数：17 个 ?（SET 14 列 + WHERE id + WHERE old_revision + WHERE organization_id）
func (r Repository) Update(ctx context.Context, o *domain.Order) error {
	var idempotencyKey interface{}
	if o.IdempotencyKey != "" {
		idempotencyKey = o.IdempotencyKey
	}

	res, err := r.db.ExecContext(ctx, queryUpdateOrder,
		o.DisplayName, o.User, o.Payment, o.Price, o.Currency,
		o.State, o.Message, o.PaymentURL, o.TradeNo,
		o.CouponName, o.CouponDiscount, idempotencyKey,
		o.UpdatedAt, o.PaidAt,
		o.ID, o.Revision-1, o.OrganizationID,
	)
	n, err := rowsAffected(res, err)
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("order revision conflict")
	}

	// 回读新 revision
	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectOrderRevision, o.ID, o.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("order not found")
		}
		return err
	}
	o.Revision = newRevision
	return nil
}

// Delete 只在订单属于该组织时删除；0 行受影响统一 NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteOrder, id, organizationID)
	if err != nil {
		return err
	}
	n, err := rowsAffected(res, err)
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("order not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
