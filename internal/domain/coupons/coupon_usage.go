package coupons

import "time"

// CouponUsage records a single use of a coupon by a user.
type CouponUsage struct {
	ID        string    `json:"id"`
	CouponID  string    `json:"couponId"`
	UserID    string    `json:"userId"`
	OrderID   string    `json:"orderId,omitempty"`
	UsedAt    time.Time `json:"usedAt"`
	CreatedAt time.Time `json:"createdAt"`
}
