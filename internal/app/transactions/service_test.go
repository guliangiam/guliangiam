package transactions

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	repoMock "github.com/guliangiam/guliangiam/internal/domain/transactions/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"go.uber.org/mock/gomock"
)

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

func TestNewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	svc := NewService(fakeClock{fixedTime: time.Now()}, fakeUUIDGenerator{fixedID: "test"}, mockUOW)
	if svc.Clock == nil {
		t.Error("Clock should not be nil")
	}
}

func TestCreateTransaction_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "tx-001"},
		UOW:   mockUOW,
	}

	t.Run("missing_orgId", func(t *testing.T) {
		err := svc.CreateTransaction(context.Background(), CreateTransactionCommand{TransactionType: "payment"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("missing_type", func(t *testing.T) {
		err := svc.CreateTransaction(context.Background(), CreateTransactionCommand{OrganizationID: "org-1"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestCreateTransaction_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Transactions().Return(mockTxRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockTxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "tx-001"},
		UOW:   mockUOW,
	}

	err := svc.CreateTransaction(context.Background(), CreateTransactionCommand{
		OrganizationID:  "org-1",
		TransactionType: "payment",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTransactions_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	now := time.Now()
	txs := []*domain.Transaction{
		{ID: "tx-1", TransactionType: "payment", CreatedAt: now, UpdatedAt: now},
		{ID: "tx-2", TransactionType: "refund", CreatedAt: now, UpdatedAt: now},
	}

	mockUOW.EXPECT().Transactions().Return(mockTxRepo).AnyTimes()
	mockTxRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(txs, int64(2), nil)

	svc := Service{UOW: mockUOW}

	items, total, err := svc.ListTransactions(context.Background(), ListTransactionsQuery{
		Page:           1,
		PageSize:       50,
		OrganizationID: "org-001",
	})
	if err != nil {
		t.Fatalf("ListTransactions() returned unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want %d", total, 2)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want %d", len(items), 2)
	}
}

func TestGetTransaction_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	now := time.Now()
	tx := &domain.Transaction{ID: "tx-1", OrganizationID: "org-1", TransactionType: "payment", CreatedAt: now, UpdatedAt: now}

	mockUOW.EXPECT().Transactions().Return(mockTxRepo).AnyTimes()
	mockTxRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "tx-1", "org-1").Return(tx, nil)

	svc := Service{UOW: mockUOW}

	detail, err := svc.GetTransaction(context.Background(), "org-1", "tx-1")
	if err != nil {
		t.Fatalf("GetTransaction() returned unexpected error: %v", err)
	}
	if detail.ID != "tx-1" {
		t.Errorf("detail.ID = %q, want %q", detail.ID, "tx-1")
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Transactions().Return(mockTxRepo).AnyTimes()
	mockTxRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "nonexistent", "org-1").Return(nil, apperrors.NewNotFound("transaction not found"))

	svc := Service{UOW: mockUOW}

	detail, err := svc.GetTransaction(context.Background(), "org-1", "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if detail != nil {
		t.Errorf("expected nil detail, got %+v", detail)
	}
}
