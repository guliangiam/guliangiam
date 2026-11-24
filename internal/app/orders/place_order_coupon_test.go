package orders

import (
	"context"
	"testing"
	"time"

	coupondomain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	productdomain "github.com/guliangiam/guliangiam/internal/domain/products"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// checkoutCouponRepo is the exact-code + atomic-increment fake the checkout
// path needs (BE-C-06/07); the embedded interface keeps the rest nil.
type checkoutCouponRepo struct {
	coupondomain.Repository
	coupons []*coupondomain.Coupon
}

func (r *checkoutCouponRepo) GetByCode(_ context.Context, organizationID, code string) (*coupondomain.Coupon, error) {
	for _, c := range r.coupons {
		if c.OrganizationID == organizationID && c.Code == code {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("coupon not found")
}

func (r *checkoutCouponRepo) IncrementUsage(_ context.Context, id, _ string, now time.Time, expectedRevision int64) (*coupondomain.Coupon, error) {
	for _, c := range r.coupons {
		if c.ID != id {
			continue
		}
		if c.State != coupondomain.StateActive || c.Revision != expectedRevision ||
			(c.Quantity > 0 && c.UsedCount >= c.Quantity) {
			return nil, apperrors.NewConflict("coupon usage increment rejected")
		}
		c.UsedCount++
		c.Revision++
		c.UpdatedAt = now
		return c, nil
	}
	return nil, apperrors.NewConflict("coupon usage increment rejected")
}

type checkoutUsageRepo struct {
	coupondomain.CouponUsageRepository
	usages []*coupondomain.CouponUsage
}

func (r *checkoutUsageRepo) Create(_ context.Context, u *coupondomain.CouponUsage) error {
	r.usages = append(r.usages, u)
	return nil
}

func (r *checkoutUsageRepo) CountByCouponAndUser(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}

func activeCheckoutCoupon() *coupondomain.Coupon {
	now := time.Now()
	return &coupondomain.Coupon{
		ID: "coupon-1", OrganizationID: "org-1", Code: "SAVE20", State: coupondomain.StateActive,
		DiscountType: "percentage", Discount: 20, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
}

// BE-C-07: the coupon discount is computed server-side, capped, deducted from
// the payable amount, and the usage is consumed in the same transaction.
func TestService_PlaceOrder_CouponDiscountsPaymentAndConsumesUsage(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	couponRepo := &checkoutCouponRepo{coupons: []*coupondomain.Coupon{activeCheckoutCoupon()}}
	usageRepo := &checkoutUsageRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "order-1"},
		UOW: testUnitOfWork{
			orderRepo: repo, productRepo: &mockProductRepo{products: []*productdomain.Product{sellableProduct("p-1", "p-1", 100, "USD")}},
			couponRepo: couponRepo, couponUsageRepo: usageRepo,
		},
	}
	var chargedAmount float64
	stubCreatePayment(t, func(_ context.Context, _ string, amount float64, _, _, _, _ string) (string, string, error) {
		chargedAmount = amount
		return "https://pay.example.com/trade-1", "trade-1", nil
	})

	detail, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", User: "user-1",
		Products: []string{"p-1"}, Currency: "USD",
		CouponName: "SAVE20",
		// A hostile client claim must be ignored, not trusted.
		CouponDiscount: 99,
	})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if detail.CouponDiscount != 20 {
		t.Fatalf("CouponDiscount = %v, want server-computed 20", detail.CouponDiscount)
	}
	if detail.Price != 80 {
		t.Fatalf("Price = %v, want discounted 80", detail.Price)
	}
	if chargedAmount != 80 {
		t.Fatalf("gateway charged %v, want the discounted 80", chargedAmount)
	}
	if couponRepo.coupons[0].UsedCount != 1 {
		t.Fatalf("coupon UsedCount = %d, want 1", couponRepo.coupons[0].UsedCount)
	}
	if len(usageRepo.usages) != 1 || usageRepo.usages[0].OrderID != "order-1" {
		t.Fatalf("usage row missing or unbound: %+v", usageRepo.usages)
	}
}

// BE-C-07: an invalid coupon rejects the whole order; nothing is written.
func TestService_PlaceOrder_InvalidCouponRejectsOrder(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	couponRepo := &checkoutCouponRepo{coupons: []*coupondomain.Coupon{activeCheckoutCoupon()}}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "order-1"},
		UOW: testUnitOfWork{
			orderRepo: repo, productRepo: &mockProductRepo{products: []*productdomain.Product{sellableProduct("p-1", "p-1", 100, "USD")}},
			couponRepo: couponRepo, couponUsageRepo: &checkoutUsageRepo{},
		},
	}
	stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
		t.Fatal("gateway must not be called for a rejected coupon")
		return "", "", nil
	})
	_, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", User: "user-1",
		Products: []string{"p-1"}, Currency: "USD",
		CouponName: "SAVE2", // prefix of SAVE20 — must not match
	})
	if err == nil {
		t.Fatal("expected rejection for a non-matching coupon code")
	}
	if apperrors.FromError(err).Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("code = %q, want unprocessable_entity", apperrors.FromError(err).Code)
	}
	if len(repo.orders) != 0 {
		t.Fatalf("order written despite coupon rejection: %d", len(repo.orders))
	}
	if couponRepo.coupons[0].UsedCount != 0 {
		t.Fatal("coupon consumed despite order rejection")
	}
}

// BE-C-07: a client-supplied discount without a coupon code is dropped.
func TestService_PlaceOrder_ClientDiscountWithoutCouponIsDropped(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "order-1"},
		UOW: testUnitOfWork{
			orderRepo: repo, productRepo: &mockProductRepo{products: []*productdomain.Product{sellableProduct("p-1", "p-1", 100, "USD")}},
		},
	}
	var chargedAmount float64
	stubCreatePayment(t, func(_ context.Context, _ string, amount float64, _, _, _, _ string) (string, string, error) {
		chargedAmount = amount
		return "https://pay.example.com/trade-1", "trade-1", nil
	})
	detail, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", User: "user-1",
		Products: []string{"p-1"}, Currency: "USD",
		CouponDiscount: 90,
	})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if detail.CouponDiscount != 0 || detail.Price != 100 || chargedAmount != 100 {
		t.Fatalf("unverified discount leaked into pricing: discount=%v price=%v charged=%v", detail.CouponDiscount, detail.Price, chargedAmount)
	}
}

// BE-C-06: an exhausted coupon rejects checkout atomically.
func TestService_PlaceOrder_ExhaustedCouponRejectsOrder(t *testing.T) {
	now := time.Now()
	c := activeCheckoutCoupon()
	c.Quantity = 1
	c.UsedCount = 1
	repo := &mockOrderRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "order-1"},
		UOW: testUnitOfWork{
			orderRepo: repo, productRepo: &mockProductRepo{products: []*productdomain.Product{sellableProduct("p-1", "p-1", 100, "USD")}},
			couponRepo: &checkoutCouponRepo{coupons: []*coupondomain.Coupon{c}}, couponUsageRepo: &checkoutUsageRepo{},
		},
	}
	_, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", User: "user-1",
		Products: []string{"p-1"}, Currency: "USD", CouponName: "SAVE20",
	})
	if err == nil {
		t.Fatal("expected rejection for an exhausted coupon")
	}
	if len(repo.orders) != 0 {
		t.Fatal("order written despite exhausted coupon")
	}
}
