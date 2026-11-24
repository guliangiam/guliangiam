package coupons

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// UsageRepository 是 MySQL 8.0+ 实现的 coupon_usage 存储层。
type UsageRepository struct {
	db mysqlshared.DBTX
}

// NewUsageRepository 接受 *sql.DB 或 *sql.Tx 构造 UsageRepository。
func NewUsageRepository(db mysqlshared.DBTX) UsageRepository {
	return UsageRepository{db: db}
}

// couponUsageCols: 6 列
func couponUsageCols() []string {
	return []string{"id", "coupon_id", "user_id", "order_id", "used_at", "created_at"}
}

// Create 插入 CouponUsage。
func (r UsageRepository) Create(ctx context.Context, u *domain.CouponUsage) error {
	_, err := r.db.ExecContext(ctx, queryUsageInsert,
		u.ID, u.CouponID, u.UserID, u.OrderID, u.UsedAt, u.CreatedAt,
	)
	return err
}

// ListByCoupon 分页查 coupon 的使用记录。
func (r UsageRepository) ListByCoupon(ctx context.Context, couponID string, page, pageSize int) ([]*domain.CouponUsage, int64, error) {
	pageSize = mysqlshared.ClampPageSize(pageSize)
	offset := mysqlshared.PageOffset(page, pageSize)

	rows, err := r.db.QueryContext(ctx, queryUsageListByCoupon, couponID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.CouponUsage, 0)
	for rows.Next() {
		var u domain.CouponUsage
		if err := rows.Scan(
			&u.ID, &u.CouponID, &u.UserID, &u.OrderID, &u.UsedAt, &u.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, &u)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, queryUsageCountByCoupon, couponID).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CountByCouponAndUser 统计某用户对某 coupon 的使用次数。
func (r UsageRepository) CountByCouponAndUser(ctx context.Context, couponID, userID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, queryUsageCountByUser, couponID, userID).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	return count, nil
}

// 编译期接口断言
var _ domain.CouponUsageRepository = UsageRepository{}
