package coupons

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
)

const (
	queryUsageInsert        = `INSERT INTO "guliangiam".coupon_usage (id, coupon_id, user_id, order_id, used_at, created_at) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6)`
	queryUsageListByCoupon  = `SELECT id, coupon_id, user_id, order_id, used_at, created_at FROM "guliangiam".coupon_usage WHERE coupon_id = $1::uuid ORDER BY used_at DESC LIMIT $2 OFFSET $3`
	queryUsageCountByCoupon = `SELECT COUNT(*) FROM "guliangiam".coupon_usage WHERE coupon_id = $1::uuid`
	queryUsageCountByUser   = `SELECT COUNT(*) FROM "guliangiam".coupon_usage WHERE coupon_id = $1::uuid AND user_id = $2::uuid`
)

type CouponUsageRepository struct{ db shared.DBTX }

func NewCouponUsageRepository(db shared.DBTX) CouponUsageRepository {
	return CouponUsageRepository{db: db}
}

func scanCouponUsage(sc interface{ Scan(...interface{}) error }) (*domain.CouponUsage, error) {
	var u domain.CouponUsage
	err := sc.Scan(&u.ID, &u.CouponID, &u.UserID, &u.OrderID, &u.UsedAt, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r CouponUsageRepository) Create(ctx context.Context, u *domain.CouponUsage) error {
	_, err := r.db.Exec(ctx, queryUsageInsert, u.ID, u.CouponID, u.UserID, u.OrderID, u.UsedAt, u.CreatedAt)
	return err
}

func (r CouponUsageRepository) ListByCoupon(ctx context.Context, couponID string, page, pageSize int) ([]*domain.CouponUsage, int64, error) {
	pageSize = shared.ClampPageSize(pageSize)
	offset := shared.PageOffset(page, pageSize)
	rows, err := r.db.Query(ctx, queryUsageListByCoupon, couponID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.CouponUsage, 0)
	for rows.Next() {
		u, err := scanCouponUsage(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, u)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryUsageCountByCoupon, couponID).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r CouponUsageRepository) CountByCouponAndUser(ctx context.Context, couponID, userID string) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, queryUsageCountByUser, couponID, userID).Scan(&count)
	if err != nil && err != pgx.ErrNoRows {
		return 0, err
	}
	return count, nil
}

var _ domain.CouponUsageRepository = CouponUsageRepository{}
