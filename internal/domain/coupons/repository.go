package coupons

import (
	"context"
	"time"
)

// ListQuery defines the query parameters for listing coupons.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	State                   *string
	DiscountType            string
}

// Repository defines the interface for coupons persistence operations.
type Repository interface {
	// GetByID is the system-scoped lookup used by order settlement
	// (IncrementCouponUsage), which is outside the management by-ID tenant
	// binding. Management routes must use GetByIDForOrganization so a
	// caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Coupon, error)
	// GetByIDForOrganization returns the coupon only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Coupon, error)
	List(ctx context.Context, q ListQuery) ([]*Coupon, int64, error)
	Create(ctx context.Context, coupon *Coupon) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, coupon *Coupon) error
	// Delete removes the coupon only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}

// CodeLookup is the exact-code lookup capability used by checkout validation
// (BE-C-07). It is a narrow optional port — satisfied by the PostgreSQL
// repository — so a fuzzy keyword search can never stand in for an exact
// coupon-code match. Dialect mirrors that do not implement it fail closed.
type CodeLookup interface {
	// GetByCode returns the coupon whose code equals code exactly within
	// organizationID; a foreign or missing row is reported as not found.
	GetByCode(ctx context.Context, organizationID, code string) (*Coupon, error)
}

// UsageIncrementer is the atomic usage-increment capability (BE-C-06). The
// increment, the total-quantity guard and the per-user guard are evaluated in
// one statement so concurrent checkouts cannot lose updates or oversell the
// coupon. expectedRevision is the revision observed by the caller's read; a
// concurrent modification rejects the increment instead of overwriting it.
type UsageIncrementer interface {
	IncrementUsage(ctx context.Context, id, userID string, now time.Time, expectedRevision int64) (*Coupon, error)
}

// CouponUsageRepository defines the interface for coupon usage persistence operations.
type CouponUsageRepository interface {
	Create(ctx context.Context, usage *CouponUsage) error
	ListByCoupon(ctx context.Context, couponID string, page, pageSize int) ([]*CouponUsage, int64, error)
	CountByCouponAndUser(ctx context.Context, couponID, userID string) (int64, error)
}
