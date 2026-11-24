package coupons

import (
	"context"
	"testing"
	"time"

	coupondomain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-C-07: a keyword prefix must never validate against a longer code.
func TestService_ValidateCoupon_ExactCodeOnly(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{
		{ID: "c-1", OrganizationID: "org-1", Code: "SUMMER20", State: coupondomain.StateActive,
			DiscountType: "percentage", Discount: 20, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{couponRepo: repo, couponUsageRepo: &mockCouponUsageRepo{}}}
	r, err := svc.ValidateCoupon(context.Background(), "org-1", "SUM", "", nil, 100)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if r.Valid {
		t.Fatalf("prefix code SUM must not match SUMMER20, got %+v", r)
	}
	r, err = svc.ValidateCoupon(context.Background(), "org-1", "SUMMER20", "", nil, 100)
	if err != nil || !r.Valid {
		t.Fatalf("exact code must validate, got %+v err=%v", r, err)
	}
}

func TestService_ValidateCoupon_CrossTenantNotFound(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{
		{ID: "c-1", OrganizationID: "org-2", Code: "SAVE10", State: coupondomain.StateActive,
			DiscountType: "fixed", Discount: 10, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{couponRepo: repo, couponUsageRepo: &mockCouponUsageRepo{}}}
	r, err := svc.ValidateCoupon(context.Background(), "org-1", "SAVE10", "", nil, 100)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if r.Valid {
		t.Fatal("foreign-tenant coupon must not validate")
	}
}

func TestService_ValidateCoupon_Exhausted(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{
		{ID: "c-1", OrganizationID: "org-1", Code: "SAVE10", State: coupondomain.StateActive,
			DiscountType: "fixed", Discount: 10, Quantity: 1, UsedCount: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{couponRepo: repo, couponUsageRepo: &mockCouponUsageRepo{}}}
	r, err := svc.ValidateCoupon(context.Background(), "org-1", "SAVE10", "", nil, 100)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if r.Valid || r.Message != "coupon usage limit reached" {
		t.Fatalf("exhausted coupon must be rejected, got %+v", r)
	}
}

// BE-C-06: the increment must be guarded by quantity and revision.
func TestService_IncrementCouponUsage_QuantityGuard(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{
		{ID: "c-1", OrganizationID: "org-1", State: coupondomain.StateActive, Quantity: 1, UsedCount: 1, Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	usageRepo := &mockCouponUsageRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "uid"},
		UOW: testUnitOfWork{couponRepo: repo, couponUsageRepo: usageRepo}}
	err := svc.IncrementCouponUsage(context.Background(), "c-1", "u-1", "o-1")
	if err == nil {
		t.Fatal("expected conflict when quantity exhausted")
	}
	if apperrors.FromError(err).Code != apperrors.CodeConflict {
		t.Fatalf("code = %q, want conflict", apperrors.FromError(err).Code)
	}
	if repo.coupons[0].UsedCount != 1 {
		t.Fatalf("UsedCount mutated despite guard: %d", repo.coupons[0].UsedCount)
	}
	if len(usageRepo.usages) != 0 {
		t.Fatal("usage row written despite guard")
	}
}

func TestService_IncrementCouponUsage_RevisionMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{
		{ID: "c-1", OrganizationID: "org-1", State: coupondomain.StateActive, Revision: 5, CreatedAt: now, UpdatedAt: now},
	}}
	// A stale expected revision (concurrent writer already bumped the row) must
	// be rejected without mutating the stored aggregate.
	_, err := repo.IncrementUsage(context.Background(), "c-1", "u-1", now, 1)
	if err == nil {
		t.Fatal("expected conflict on revision mismatch")
	}
	if apperrors.FromError(err).Code != apperrors.CodeConflict {
		t.Fatalf("code = %q, want conflict", apperrors.FromError(err).Code)
	}
	if repo.coupons[0].UsedCount != 0 || repo.coupons[0].Revision != 5 {
		t.Fatalf("row mutated despite revision guard: used=%d rev=%d", repo.coupons[0].UsedCount, repo.coupons[0].Revision)
	}
}

func TestService_IncrementCouponUsage_DisabledRejected(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{
		{ID: "c-1", OrganizationID: "org-1", State: coupondomain.StateDisabled, Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "uid"},
		UOW: testUnitOfWork{couponRepo: repo, couponUsageRepo: &mockCouponUsageRepo{}}}
	if err := svc.IncrementCouponUsage(context.Background(), "c-1", "u-1", "o-1"); err == nil {
		t.Fatal("disabled coupon must not be consumable")
	}
}

// BE-C-07: checkout discount is computed server-side and capped.
func TestResolveCheckoutCoupon_DiscountCapped(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		coupon   coupondomain.Coupon
		amount   float64
		expected float64
	}{
		{name: "percentage", coupon: coupondomain.Coupon{DiscountType: "percentage", Discount: 20}, amount: 100, expected: 20},
		{name: "percentage maxDiscount cap", coupon: coupondomain.Coupon{DiscountType: "percentage", Discount: 50, MaxDiscount: 10}, amount: 100, expected: 10},
		{name: "fixed above total capped", coupon: coupondomain.Coupon{DiscountType: "fixed", Discount: 500}, amount: 100, expected: 100},
		{name: "fixed", coupon: coupondomain.Coupon{DiscountType: "fixed", Discount: 15}, amount: 100, expected: 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.coupon
			c.ID = "c-1"
			c.OrganizationID = "org-1"
			c.Code = "SAVE"
			c.State = coupondomain.StateActive
			c.CreatedAt = now
			c.UpdatedAt = now
			repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{&c}}
			tx := testUnitOfWork{couponRepo: repo, couponUsageRepo: &mockCouponUsageRepo{}}
			discount, coupon, err := ResolveCheckoutCoupon(context.Background(), tx, fakeClock{fixedTime: now}, "org-1", "SAVE", "u-1", nil, tt.amount)
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if coupon == nil || coupon.ID != "c-1" || discount != tt.expected {
				t.Fatalf("discount = %v coupon = %+v, want %v / c-1", discount, coupon, tt.expected)
			}
		})
	}
}

func TestResolveCheckoutCoupon_Rejections(t *testing.T) {
	now := time.Now()
	base := coupondomain.Coupon{ID: "c-1", OrganizationID: "org-1", Code: "SAVE", State: coupondomain.StateActive,
		DiscountType: "fixed", Discount: 10, CreatedAt: now, UpdatedAt: now}
	tests := []struct {
		name   string
		mutate func(*coupondomain.Coupon)
		code   string
	}{
		{name: "unknown code", code: "NOPE"},
		{name: "prefix code", code: "SAV"},
		{name: "disabled", mutate: func(c *coupondomain.Coupon) { c.State = coupondomain.StateDisabled }, code: "SAVE"},
		{name: "exhausted", mutate: func(c *coupondomain.Coupon) { c.Quantity = 1; c.UsedCount = 1 }, code: "SAVE"},
		{name: "expired", mutate: func(c *coupondomain.Coupon) { c.ExpireTime = now.Add(-time.Hour).Format(time.RFC3339) }, code: "SAVE"},
		{name: "below minimum", mutate: func(c *coupondomain.Coupon) { c.MinOrderAmount = 200 }, code: "SAVE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base
			if tt.mutate != nil {
				tt.mutate(&c)
			}
			repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{&c}}
			tx := testUnitOfWork{couponRepo: repo, couponUsageRepo: &mockCouponUsageRepo{}}
			_, _, err := ResolveCheckoutCoupon(context.Background(), tx, fakeClock{fixedTime: now}, "org-1", tt.code, "u-1", nil, 100)
			if err == nil {
				t.Fatal("expected rejection")
			}
			if apperrors.FromError(err).Code != apperrors.CodeUnprocessableEntity {
				t.Fatalf("code = %q, want unprocessable_entity", apperrors.FromError(err).Code)
			}
		})
	}
}

// BE-C-06: RecordUsageInTx persists the increment and the per-user usage row.
func TestRecordUsageInTx_PersistsIncrementAndUsage(t *testing.T) {
	now := time.Now()
	c := &coupondomain.Coupon{ID: "c-1", OrganizationID: "org-1", State: coupondomain.StateActive, Revision: 1, CreatedAt: now, UpdatedAt: now}
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{c}}
	usageRepo := &mockCouponUsageRepo{}
	tx := testUnitOfWork{couponRepo: repo, couponUsageRepo: usageRepo}
	if err := RecordUsageInTx(context.Background(), tx, fakeClock{fixedTime: now}, fakeUUIDGenerator{fixedID: "usage-1"}, c, "u-1", "o-1"); err != nil {
		t.Fatalf("error = %v", err)
	}
	if repo.coupons[0].UsedCount != 1 || repo.coupons[0].Revision != 2 {
		t.Fatalf("increment not persisted: used=%d rev=%d", repo.coupons[0].UsedCount, repo.coupons[0].Revision)
	}
	if len(usageRepo.usages) != 1 || usageRepo.usages[0].OrderID != "o-1" {
		t.Fatalf("usage row missing: %+v", usageRepo.usages)
	}
}

func TestDiscountAmount_NeverNegativeOrAboveTotal(t *testing.T) {
	c := &coupondomain.Coupon{DiscountType: "fixed", Discount: -5}
	if d := DiscountAmount(c, 100); d != 0 {
		t.Fatalf("negative discount must clamp to 0, got %v", d)
	}
	c = &coupondomain.Coupon{DiscountType: "percentage", Discount: 100}
	if d := DiscountAmount(c, 80); d != 80 {
		t.Fatalf("100%% of 80 must be 80, got %v", d)
	}
}
