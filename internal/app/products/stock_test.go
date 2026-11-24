package products

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/products"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// stockCall records one atomic ledger invocation.
type stockCall struct {
	id       string
	orgID    string
	quantity int
	release  bool
}

// fakeLedgerRepo implements the StockLedger capability over the products port.
type fakeLedgerRepo struct {
	domain.Repository
	calls []stockCall
	err   error
}

func (r *fakeLedgerRepo) IncrementSold(_ context.Context, id, orgID string, quantity int, _ time.Time) (*domain.Product, error) {
	r.calls = append(r.calls, stockCall{id: id, orgID: orgID, quantity: quantity})
	if r.err != nil {
		return nil, r.err
	}
	return &domain.Product{ID: id}, nil
}

func (r *fakeLedgerRepo) ReleaseSold(_ context.Context, id, orgID string, quantity int, _ time.Time) (*domain.Product, error) {
	r.calls = append(r.calls, stockCall{id: id, orgID: orgID, quantity: quantity, release: true})
	if r.err != nil {
		return nil, r.err
	}
	return &domain.Product{ID: id}, nil
}

// stockUoW serves only the Products() capability the helpers touch.
type stockUoW struct {
	appuow.UnitOfWork
	repo domain.Repository
}

func (u stockUoW) Products() domain.Repository { return u.repo }

// TestBasketStock_AggregatesDuplicateIDs pins BE-C-11: a basket listing the
// same product N times consumes N units in one atomic increment per product.
func TestBasketStock_AggregatesDuplicateIDs(t *testing.T) {
	repo := &fakeLedgerRepo{}
	uow := stockUoW{repo: repo}
	clk := fakeClock{fixedTime: time.Now()}

	if err := ReserveBasketInTx(context.Background(), uow, clk, "org-1", []string{"p-1", "p-2", "p-1"}); err != nil {
		t.Fatalf("ReserveBasketInTx() error = %v", err)
	}
	want := []stockCall{{id: "p-1", orgID: "org-1", quantity: 2}, {id: "p-2", orgID: "org-1", quantity: 1}}
	if len(repo.calls) != len(want) {
		t.Fatalf("calls = %+v, want %+v", repo.calls, want)
	}
	for i, c := range repo.calls {
		if c != want[i] {
			t.Errorf("call[%d] = %+v, want %+v", i, c, want[i])
		}
	}

	repo.calls = nil
	if err := ReleaseBasketInTx(context.Background(), uow, clk, "org-1", []string{"p-1", "p-1"}); err != nil {
		t.Fatalf("ReleaseBasketInTx() error = %v", err)
	}
	if len(repo.calls) != 1 || repo.calls[0].quantity != 2 || !repo.calls[0].release {
		t.Errorf("release calls = %+v, want one aggregated release of 2", repo.calls)
	}
}

// TestBasketStock_PropagatesGuardRejection: an insufficient-stock rejection
// aborts the basket so the caller's transaction rolls back.
func TestBasketStock_PropagatesGuardRejection(t *testing.T) {
	repo := &fakeLedgerRepo{err: apperrors.NewUnprocessableEntity("insufficient product quantity")}
	uow := stockUoW{repo: repo}
	err := ReserveBasketInTx(context.Background(), uow, fakeClock{fixedTime: time.Now()}, "org-1", []string{"p-1"})
	if err == nil {
		t.Fatal("ReserveBasketInTx() error = nil, want the guard rejection propagated")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("error code = %s, want %s", code, apperrors.CodeUnprocessableEntity)
	}
}

// TestBasketStock_FailsClosedWithoutCapability: a dialect without the atomic
// ledger must not silently fall back to a racy read-check.
func TestBasketStock_FailsClosedWithoutCapability(t *testing.T) {
	uow := stockUoW{repo: &plainRepo{}}
	err := ReserveBasketInTx(context.Background(), uow, fakeClock{fixedTime: time.Now()}, "org-1", []string{"p-1"})
	if err == nil {
		t.Fatal("ReserveBasketInTx() error = nil, want fail-closed for missing capability")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeUnimplemented {
		t.Fatalf("error code = %s, want %s", code, apperrors.CodeUnimplemented)
	}
}

// plainRepo embeds the port without the StockLedger capability.
type plainRepo struct{ domain.Repository }

// TestBasketStock_EmptyBasketIsNoop keeps the helpers safe for orders without
// catalog products.
func TestBasketStock_EmptyBasketIsNoop(t *testing.T) {
	uow := stockUoW{repo: &plainRepo{}}
	if err := ReserveBasketInTx(context.Background(), uow, fakeClock{fixedTime: time.Now()}, "org-1", nil); err != nil {
		t.Fatalf("ReserveBasketInTx(empty) error = %v", err)
	}
	if err := ReleaseBasketInTx(context.Background(), uow, fakeClock{fixedTime: time.Now()}, "org-1", nil); err != nil {
		t.Fatalf("ReleaseBasketInTx(empty) error = %v", err)
	}
}
