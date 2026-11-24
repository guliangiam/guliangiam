package coupons

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 coupons 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// couponRow 是 coupons 表扫描目标。
// Products / Users PG jsonb → MySQL TEXT (JSON 字符串)
type couponRow struct {
	ID              string
	OrganizationID  string
	Name            string
	DisplayName     string
	Description     string
	Code            string
	DiscountType    string
	Discount        float64
	MaxDiscount     float64
	Scope           string
	Products        string // JSON → []string
	Users           string // JSON → []string
	Quantity        int
	UsedCount       int
	MaxUsagePerUser int
	StartTime       string
	ExpireTime      string
	MinOrderAmount  float64
	Currency        string
	State           string
	CreatedAt       interface{}
	UpdatedAt       interface{}
	Revision        int64
}

// couponFullScan 扫描完整 coupon（22 列）。
func couponFullScan(scanner interface{ Scan(...any) error }) (*domain.Coupon, error) {
	var row couponRow
	err := scanner.Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Description,
		&row.Code, &row.DiscountType, &row.Discount, &row.MaxDiscount, &row.Scope,
		&row.Products, &row.Users, &row.Quantity, &row.UsedCount, &row.MaxUsagePerUser,
		&row.StartTime, &row.ExpireTime, &row.MinOrderAmount, &row.Currency, &row.State,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		return nil, err
	}
	return toCoupon(row)
}

// toCoupon 把 MySQL row 转成 domain.Coupon。
func toCoupon(row couponRow) (*domain.Coupon, error) {
	c := &domain.Coupon{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		Name:            row.Name,
		DisplayName:     row.DisplayName,
		Description:     row.Description,
		Code:            row.Code,
		DiscountType:    row.DiscountType,
		Discount:        row.Discount,
		MaxDiscount:     row.MaxDiscount,
		Scope:           row.Scope,
		Quantity:        row.Quantity,
		UsedCount:       row.UsedCount,
		MaxUsagePerUser: row.MaxUsagePerUser,
		StartTime:       row.StartTime,
		ExpireTime:      row.ExpireTime,
		MinOrderAmount:  row.MinOrderAmount,
		Currency:        row.Currency,
		State:           row.State,
		Revision:        row.Revision,
	}
	if row.Products != "" && row.Products != "null" {
		if err := json.Unmarshal([]byte(row.Products), &c.Products); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal products")
		}
	}
	if c.Products == nil {
		c.Products = []string{}
	}
	if row.Users != "" && row.Users != "null" {
		if err := json.Unmarshal([]byte(row.Users), &c.Users); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal users")
		}
	}
	if c.Users == nil {
		c.Users = []string{}
	}
	return c, nil
}

// listArgs 构造 List 查询的参数。
//
//	org_id×2, keyword×4, state×2, discount_type×2, limit, offset = 12
func listCouponArgs(q domain.ListQuery) []any {
	stateArg := any(nil)
	if q.State != nil {
		stateArg = *q.State
	}
	return []any{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword, q.Keyword,
		stateArg, stateArg,
		q.DiscountType, q.DiscountType,
		mysqlshared.ClampPageSize(q.PageSize), mysqlshared.PageOffset(q.Page, q.PageSize),
	}
}

// countCouponArgs 跟 listArgs 相同，只是去掉最后 limit/offset。
func countCouponArgs(q domain.ListQuery) []any {
	stateArg := any(nil)
	if q.State != nil {
		stateArg = *q.State
	}
	return []any{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword, q.Keyword,
		stateArg, stateArg,
		q.DiscountType, q.DiscountType,
	}
}

// GetByID 按 id 查 Coupon（系统路径，不带请求租户）。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Coupon, error) {
	return r.getByIDQuery(ctx, queryGetByID, id)
}

// GetByIDForOrganization 按 id 查 Coupon（organization_id 谓词防止跨租户读取）。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Coupon, error) {
	return r.getByIDQuery(ctx, queryGetByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Coupon, error) {
	c, err := couponFullScan(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("coupon not found")
		}
		return nil, err
	}
	return c, nil
}

// List 分页查询 Coupons。
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Coupon, int64, error) {
	rows, err := r.db.QueryContext(ctx, queryList, listCouponArgs(q)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Coupon, 0)
	for rows.Next() {
		var c domain.Coupon
		if err := rows.Scan(
			&c.ID, &c.OrganizationID, &c.Name, &c.DisplayName, &c.Code,
			&c.DiscountType, &c.Discount, &c.State, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, &c)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCount, countCouponArgs(q)...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create 插入 Coupon。
func (r Repository) Create(ctx context.Context, c *domain.Coupon) error {
	productsJSON, err := json.Marshal(c.Products)
	if err != nil {
		return apperrors.NewInternal("encode products")
	}
	usersJSON, err := json.Marshal(c.Users)
	if err != nil {
		return apperrors.NewInternal("encode users")
	}

	_, err = r.db.ExecContext(ctx, queryInsert,
		c.ID, c.OrganizationID, c.Name, c.DisplayName, c.Description, c.Code,
		c.DiscountType, c.Discount, c.MaxDiscount, c.Scope, string(productsJSON), string(usersJSON),
		c.Quantity, c.UsedCount, c.MaxUsagePerUser, c.StartTime, c.ExpireTime,
		c.MinOrderAmount, c.Currency, c.State, c.CreatedAt, c.UpdatedAt, c.Revision,
	)
	return err
}

// Update 更新 Coupon。
func (r Repository) Update(ctx context.Context, c *domain.Coupon) error {
	productsJSON, err := json.Marshal(c.Products)
	if err != nil {
		return apperrors.NewInternal("encode products")
	}
	usersJSON, err := json.Marshal(c.Users)
	if err != nil {
		return apperrors.NewInternal("encode users")
	}

	res, err := r.db.ExecContext(ctx, queryUpdate,
		c.DisplayName, c.Description, c.Discount, c.MaxDiscount, c.Scope,
		string(productsJSON), string(usersJSON),
		c.Quantity, c.MaxUsagePerUser, c.StartTime, c.ExpireTime,
		c.MinOrderAmount, c.Currency, c.State, c.UpdatedAt, c.Revision,
		c.ID, c.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// 实体已在 app 层按租户读取：0 affected 表示行已被并发删除。
		return apperrors.NewNotFound("coupon not found")
	}
	return nil
}

// Delete 按 id 删除 Coupon（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDelete, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("coupon not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
