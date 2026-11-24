package transactions

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	repoMock "github.com/guliangiam/guliangiam/internal/domain/transactions/mock"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestRecordTransaction_MissingOrgID(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "tx-001"},
	}
	_, err := svc.RecordTransaction(context.Background(), "", "user-1", "", "", "payment", 100.0, "USD", "", "", nil)
	assert.Error(t, err)
}

func TestRecordTransaction_MissingTransactionType(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "tx-001"},
	}
	_, err := svc.RecordTransaction(context.Background(), "org-1", "user-1", "", "", "", 100.0, "USD", "", "", nil)
	assert.Error(t, err)
}

func TestRecordTransaction_Success(t *testing.T) {
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

	result, err := svc.RecordTransaction(context.Background(), "org-1", "user-1", "order-1", "pay-1", "payment", 100.0, "USD", "stripe", "provider-1", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "tx-001", result.ID)
	assert.Equal(t, "org-1", result.OrganizationID)
	assert.Equal(t, "payment", result.TransactionType)
}

func TestRecordTransaction_WithMetadata(t *testing.T) {
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
		UUID:  fakeUUIDGenerator{fixedID: "tx-002"},
		UOW:   mockUOW,
	}

	meta := json.RawMessage(`{"key":"value"}`)
	result, err := svc.RecordTransaction(context.Background(), "org-1", "user-1", "", "", "refund", 50.0, "USD", "", "", meta)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGetOrderTransactions_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Transactions().Return(mockTxRepo)
	mockTxRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domain.Transaction{
		{ID: "tx-1", TransactionType: "payment", Amount: 100, Currency: "USD", Status: "completed"},
	}, int64(1), nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "tx-001"},
		UOW:   mockUOW,
	}

	items, total, err := svc.GetOrderTransactions(context.Background(), "org-1", "order-1", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
}

func TestGetOrderTransactions_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Transactions().Return(mockTxRepo)
	mockTxRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "tx-001"},
		UOW:   mockUOW,
	}

	_, _, err := svc.GetOrderTransactions(context.Background(), "org-1", "order-1", 1, 10)
	assert.Error(t, err)
}

func TestRefundTransaction_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	now := time.Now()
	originalTx := &domain.Transaction{
		ID:              "tx-original",
		OrganizationID:  "org-1",
		UserID:          "user-1",
		OrderID:         "order-1",
		PaymentID:       "pay-1",
		TransactionType: "payment",
		Amount:          100.0,
		Status:          "completed",
		Currency:        "USD",
		ProviderType:    "stripe",
		ProviderID:      "provider-1",
	}

	mockUOW.EXPECT().Transactions().Return(mockTxRepo).AnyTimes()
	mockTxRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "tx-original", "org-1").Return(originalTx, nil)
	mockTxRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockTxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "tx-refund"},
		UOW:   mockUOW,
	}

	result, err := svc.RefundTransaction(context.Background(), "org-1", "tx-original", 50.0, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "refund", result.TransactionType)
	assert.Equal(t, 50.0, result.Amount)
}

func TestRefundTransaction_OriginalNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Transactions().Return(mockTxRepo)
	mockTxRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "nonexistent", "org-1").Return(nil, assert.AnError)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "tx-refund"},
		UOW:   mockUOW,
	}

	_, err := svc.RefundTransaction(context.Background(), "org-1", "nonexistent", 50.0, nil)
	assert.Error(t, err)
}

func TestRefundTransaction_NotPaymentType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	originalTx := &domain.Transaction{
		ID:              "tx-1",
		TransactionType: "refund",
		Status:          "completed",
	}

	mockUOW.EXPECT().Transactions().Return(mockTxRepo)
	mockTxRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "tx-1", "org-1").Return(originalTx, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "tx-refund"},
		UOW:   mockUOW,
	}

	_, err := svc.RefundTransaction(context.Background(), "org-1", "tx-1", 50.0, nil)
	assert.Error(t, err)
}

func TestRefundTransaction_NotCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	originalTx := &domain.Transaction{
		ID:              "tx-1",
		TransactionType: "payment",
		Status:          "pending",
	}

	mockUOW.EXPECT().Transactions().Return(mockTxRepo)
	mockTxRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "tx-1", "org-1").Return(originalTx, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "tx-refund"},
		UOW:   mockUOW,
	}

	_, err := svc.RefundTransaction(context.Background(), "org-1", "tx-1", 50.0, nil)
	assert.Error(t, err)
}

func TestRefundTransaction_WithMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)

	originalTx := &domain.Transaction{
		ID:              "tx-original",
		OrganizationID:  "org-1",
		UserID:          "user-1",
		OrderID:         "order-1",
		PaymentID:       "pay-1",
		TransactionType: "payment",
		Amount:          100.0,
		Status:          "completed",
		Currency:        "USD",
		ProviderType:    "stripe",
		ProviderID:      "provider-1",
	}

	mockUOW.EXPECT().Transactions().Return(mockTxRepo).AnyTimes()
	mockTxRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "tx-original", "org-1").Return(originalTx, nil)
	mockTxRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockTxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "tx-refund"},
		UOW:   mockUOW,
	}

	meta := json.RawMessage(`{"reason":"customer_request"}`)
	result, err := svc.RefundTransaction(context.Background(), "org-1", "tx-original", 50.0, meta)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}
