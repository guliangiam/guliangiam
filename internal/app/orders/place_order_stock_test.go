package orders

import (
	"context"
	"testing"
	"time"

	productdomain "github.com/guliangiam/guliangiam/internal/domain/products"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// stockProduct is a sellable catalog product with an explicit stock ledger.
func stockProduct(id string, quantity, sold int) *productdomain.Product {
	p := sellableProduct(id, id, 10, "USD")
	p.Quantity = quantity
	p.Sold = sold
	return p
}

func newStockService(repo *mockOrderRepo, products ...*productdomain.Product) Service {
	return Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &fakeUUIDGenerator{fixedID: "order-1"},
		UOW:   testUnitOfWork{orderRepo: repo, productRepo: &mockProductRepo{products: products}},
	}
}

// TestService_PlaceOrder_ReservesStockAtomically pins BE-C-11: placement
// consumes stock under the sold+q<=quantity guard, so the last unit can only
// be sold once even though every caller passed the advisory pre-check.
func TestService_PlaceOrder_ReservesStockAtomically(t *testing.T) {
	tests := []struct {
		name       string
		quantity   int
		sold       int
		basket     []string
		wantErr    bool
		wantSold   int
		wantOrders int
	}{
		{name: "reserves one unit", quantity: 5, sold: 0, basket: []string{"p-1"}, wantSold: 1, wantOrders: 1},
		{name: "aggregates duplicate ids", quantity: 5, sold: 0, basket: []string{"p-1", "p-1", "p-1"}, wantSold: 3, wantOrders: 1},
		{name: "last unit goes to the first buyer", quantity: 1, sold: 0, basket: []string{"p-1"}, wantSold: 1, wantOrders: 1},
		{name: "oversell rejected", quantity: 1, sold: 1, basket: []string{"p-1"}, wantErr: true, wantSold: 1, wantOrders: 0},
		{name: "basket larger than remaining stock rejected", quantity: 3, sold: 2, basket: []string{"p-1", "p-1"}, wantErr: true, wantSold: 2, wantOrders: 0},
		{name: "unlimited stock ignores quantity", quantity: 0, sold: 100, basket: []string{"p-1"}, wantSold: 101, wantOrders: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockOrderRepo{}
			product := stockProduct("p-1", tt.quantity, tt.sold)
			svc := newStockService(repo, product)
			stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
				return "https://pay.example.com/trade-1", "trade-1", nil
			})

			_, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
				OrganizationID: "org-1", User: "user-1", Products: tt.basket,
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("PlaceOrder() error = nil, want insufficient-stock rejection")
				}
				if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
					t.Fatalf("error code = %s, want %s", code, apperrors.CodeUnprocessableEntity)
				}
			} else if err != nil {
				t.Fatalf("PlaceOrder() error = %v", err)
			}
			if product.Sold != tt.wantSold {
				t.Errorf("Sold = %d, want %d", product.Sold, tt.wantSold)
			}
			if len(repo.orders) != tt.wantOrders {
				t.Errorf("orders written = %d, want %d", len(repo.orders), tt.wantOrders)
			}
		})
	}
}

// TestService_PlaceOrder_SecondBuyerCannotOversell replays the ledger repro:
// a quantity=1 product bought twice — the second placement must fail.
func TestService_PlaceOrder_SecondBuyerCannotOversell(t *testing.T) {
	repo := &mockOrderRepo{}
	product := stockProduct("p-1", 1, 0)
	svc := newStockService(repo, product)
	stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
		return "https://pay.example.com/trade-1", "trade-1", nil
	})

	if _, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", User: "buyer-1", Products: []string{"p-1"},
	}); err != nil {
		t.Fatalf("first PlaceOrder() error = %v", err)
	}
	_, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", User: "buyer-2", Products: []string{"p-1"},
	})
	if err == nil {
		t.Fatal("second PlaceOrder() error = nil, want the atomic stock guard to reject the oversell")
	}
	if product.Sold != 1 {
		t.Errorf("Sold = %d, want 1 (no double consumption)", product.Sold)
	}
	if len(repo.orders) != 1 {
		t.Errorf("orders written = %d, want 1", len(repo.orders))
	}
}

// TestService_CancelOrder_ReleasesReservedStock: an unpaid cancelled order
// returns its reservation to the catalog (BE-C-11).
func TestService_CancelOrder_ReleasesReservedStock(t *testing.T) {
	repo := &mockOrderRepo{}
	product := stockProduct("p-1", 5, 0)
	svc := newStockService(repo, product)
	// The channel never produces a trade, so the order stays `created` and
	// cancellable; the stock reservation still happened at placement.
	stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
		return "", "", apperrors.NewUnavailable("channel down")
	})

	detail, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", User: "user-1", Products: []string{"p-1", "p-1"},
	})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if product.Sold != 2 {
		t.Fatalf("Sold after placement = %d, want 2", product.Sold)
	}
	if err := svc.CancelOrder(context.Background(), "org-1", detail.ID, 0); err != nil {
		t.Fatalf("CancelOrder() error = %v", err)
	}
	if product.Sold != 0 {
		t.Errorf("Sold after cancel = %d, want 0 (reservation returned)", product.Sold)
	}
}

// TestService_DeleteOrder_ReleasesReservedStock: deleting an unpaid catalog
// order returns its reservation; a paid order's consumption is final.
func TestService_DeleteOrder_ReleasesReservedStock(t *testing.T) {
	repo := &mockOrderRepo{}
	product := stockProduct("p-1", 5, 0)
	svc := newStockService(repo, product)
	stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
		return "", "", apperrors.NewUnavailable("channel down")
	})

	detail, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", User: "user-1", Products: []string{"p-1"},
	})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if err := svc.DeleteOrder(context.Background(), "org-1", detail.ID); err != nil {
		t.Fatalf("DeleteOrder() error = %v", err)
	}
	if product.Sold != 0 {
		t.Errorf("Sold after delete = %d, want 0 (reservation returned)", product.Sold)
	}
	if len(repo.orders) != 0 {
		t.Errorf("orders remaining = %d, want 0", len(repo.orders))
	}
}
