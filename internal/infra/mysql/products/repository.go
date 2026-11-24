package products

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/products"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 products 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 是结算/购买路径使用的系统作用域查询。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	return r.getByIDQuery(ctx, queryGetProductByID, id)
}

// GetByIDForOrganization 按 id + organization_id 查 Product（跨租户 id 与不存在同为 NotFound）。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Product, error) {
	return r.getByIDQuery(ctx, queryGetProductByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Product, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	p, err := scanProduct(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("product not found")
		}
		return nil, err
	}
	return p, nil
}

// List 分页列出 Product。
// countArgs: org_id×2 + keyword×3 + state×2 + is_recharge×2 = 9
// listArgs: 同上 + limit + offset = 11
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Product, int64, error) {
	var stateArg interface{}
	if q.State != nil {
		stateArg = *q.State
	}
	var rechargeArg interface{}
	if q.IsRecharge != nil {
		rechargeArg = *q.IsRecharge
	}

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		stateArg, stateArg,
		rechargeArg, rechargeArg,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountProducts, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListProducts, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
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

// Create 插入 Product。
func (r Repository) Create(ctx context.Context, p *domain.Product) error {
	rechargeOpts, _ := json.Marshal(p.RechargeOptions)
	providers, _ := json.Marshal(p.Providers)
	_, err := r.db.ExecContext(ctx, queryInsertProduct,
		p.ID, p.OrganizationID, p.Name, p.DisplayName, p.Image, p.Detail, p.Description,
		p.Tag, p.Currency, p.Price, p.Quantity, p.Sold, p.IsRecharge, rechargeOpts,
		p.DisableCustomRecharge, providers, p.SuccessUrl, p.State,
		p.CreatedAt, p.UpdatedAt, p.Revision,
	)
	return err
}

// Update 更新 Product（organization_id 取自实体；0 行受影响 → NotFound）。
func (r Repository) Update(ctx context.Context, p *domain.Product) error {
	rechargeOpts, _ := json.Marshal(p.RechargeOptions)
	providers, _ := json.Marshal(p.Providers)
	res, err := r.db.ExecContext(ctx, queryUpdateProduct,
		p.DisplayName, p.Image, p.Detail, p.Description, p.Tag, p.Currency, p.Price,
		p.Quantity, p.Sold, p.IsRecharge, rechargeOpts, p.DisableCustomRecharge,
		providers, p.SuccessUrl, p.State, p.UpdatedAt, p.Revision, p.ID, p.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// 实体已在 app 层按租户读取：0 行只可能是同一租户内的行已消失。
		return apperrors.NewNotFound("product not found")
	}
	return nil
}

// Delete 只在 Product 属于该组织时删除；0 行受影响 → NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteProduct, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("product not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
