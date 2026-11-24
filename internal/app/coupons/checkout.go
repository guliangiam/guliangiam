package coupons

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// couponRejectionReason is the single validation rule shared by the soft
// ValidateCoupon endpoint and the hard checkout path (BE-C-07). It returns ""
// when the coupon may be used, or the human-readable rejection reason.
func couponRejectionReason(c *domain.Coupon, now time.Time, userCount int64, productNames []string, orderAmount float64) string {
	if c.State != domain.StateActive {
		return "coupon not found or inactive"
	}
	if c.ExpireTime != "" {
		if expireTime, err := time.Parse(time.RFC3339, c.ExpireTime); err == nil && now.After(expireTime) {
			return "coupon has expired"
		}
	}
	if c.StartTime != "" {
		if startTime, err := time.Parse(time.RFC3339, c.StartTime); err == nil && now.Before(startTime) {
			return "coupon is not yet active"
		}
	}
	if c.Quantity > 0 && c.UsedCount >= c.Quantity {
		return "coupon usage limit reached"
	}
	if c.MaxUsagePerUser > 0 && userCount >= int64(c.MaxUsagePerUser) {
		return "per-user usage limit reached"
	}
	if c.Scope == "Product" && len(c.Products) > 0 && len(productNames) > 0 && !couponAppliesToProducts(c.Products, productNames) {
		return "coupon is not applicable to selected products"
	}
	if c.MinOrderAmount > 0 && orderAmount < c.MinOrderAmount {
		return "order amount is below minimum required"
	}
	return ""
}

// DiscountAmount computes the money a coupon takes off orderAmount: percentage
// coupons apply their rate, fixed coupons their face value; MaxDiscount caps
// the result and the discount can never exceed the order total (BE-C-07).
func DiscountAmount(c *domain.Coupon, orderAmount float64) float64 {
	d := c.Discount
	if c.DiscountType == "percentage" {
		d = orderAmount * c.Discount / 100
	}
	if c.MaxDiscount > 0 && d > c.MaxDiscount {
		d = c.MaxDiscount
	}
	if d > orderAmount {
		d = orderAmount
	}
	if d < 0 {
		d = 0
	}
	return d
}

// ResolveCheckoutCoupon validates an exact coupon code inside the caller's
// checkout transaction and returns the capped discount plus the coupon read
// (its revision pins the subsequent atomic usage increment). Every rejection
// is a hard 422 so an invalid coupon can never silently price an order
// (BE-C-07). productNames are the catalog product names of the basket.
func ResolveCheckoutCoupon(ctx context.Context, tx appuow.UnitOfWork, clk clock.Clock, orgID, code, userID string, productNames []string, orderAmount float64) (float64, *domain.Coupon, error) {
	lookup, ok := tx.Coupons().(domain.CodeLookup)
	if !ok {
		return 0, nil, apperrors.NewUnimplemented("coupon exact-code lookup is not supported by this dialect")
	}
	c, err := lookup.GetByCode(ctx, orgID, code)
	if err != nil {
		if apperrors.FromError(err).Code == apperrors.CodeNotFound {
			return 0, nil, apperrors.NewUnprocessableEntity("coupon not found or inactive")
		}
		return 0, nil, err
	}
	var userCount int64
	if c.MaxUsagePerUser > 0 && userID != "" {
		userCount, err = tx.CouponUsages().CountByCouponAndUser(ctx, c.ID, userID)
		if err != nil {
			return 0, nil, err
		}
	}
	if reason := checkoutRejectionReason(c, clk.Now(), userID, userCount, productNames, orderAmount); reason != "" {
		return 0, nil, apperrors.NewUnprocessableEntity(reason)
	}
	return DiscountAmount(c, orderAmount), c, nil
}

// checkoutRejectionReason adapts the shared rule to a concrete caller identity.
func checkoutRejectionReason(c *domain.Coupon, now time.Time, userID string, userCount int64, productNames []string, orderAmount float64) string {
	if c.Scope == "User" && len(c.Users) > 0 && userID != "" && !couponAppliesToUser(c.Users, userID) {
		return "coupon is not applicable to this user"
	}
	return couponRejectionReason(c, now, userCount, productNames, orderAmount)
}

// RecordUsageInTx consumes one coupon use inside the caller's transaction:
// the atomic guarded increment (BE-C-06), the per-user usage row and both audit
// events commit together with the order. An empty userID (anonymous checkout)
// skips the per-user row, which the SQL guard also tolerates.
func RecordUsageInTx(ctx context.Context, tx appuow.UnitOfWork, clk clock.Clock, gen platformuuid.Generator, c *domain.Coupon, userID, orderID string) error {
	inc, ok := tx.Coupons().(domain.UsageIncrementer)
	if !ok {
		return apperrors.NewUnimplemented("coupon atomic usage increment is not supported by this dialect")
	}
	updated, err := inc.IncrementUsage(ctx, c.ID, userID, clk.Now(), c.Revision)
	if err != nil {
		return err
	}
	if err := audit.EmbedAuditEvent(
		clk, tx, ctx,
		"", "", "", "", "",
		updated.OrganizationID, "", "coupon.updated", "coupon", updated.ID,
		"coupon", "coupon.updated",
		map[string]any{},
	); err != nil {
		return err
	}
	if strings.TrimSpace(userID) == "" {
		return nil
	}
	usageID, err := gen.NewString()
	if err != nil {
		return err
	}
	now := clk.Now()
	usage := &domain.CouponUsage{
		ID:        usageID,
		CouponID:  c.ID,
		UserID:    userID,
		OrderID:   orderID,
		UsedAt:    now,
		CreatedAt: now,
	}
	if err := tx.CouponUsages().Create(ctx, usage); err != nil {
		return err
	}
	return audit.EmbedAuditEvent(
		clk, tx, ctx,
		"", "", "", "", "",
		updated.OrganizationID, "", "coupon.usage.created", "coupon_usage", usage.ID,
		"coupon_usage", "coupon.usage.created",
		map[string]any{},
	)
}
