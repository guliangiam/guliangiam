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
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// TestRecordTransaction_AmountGuard pins BE-C-15: the ledger only records
// real, positive money movements.
func TestRecordTransaction_AmountGuard(t *testing.T) {
	for _, amount := range []float64{0, -10} {
		svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "tx-1"}}
		_, err := svc.RecordTransaction(context.Background(), "org-1", "user-1", "", "", "payment", amount, "USD", "", "", nil)
		if err == nil {
			t.Fatalf("RecordTransaction(amount=%v) error = nil, want rejection", amount)
		}
		assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	}
}

// TestRecordTransaction_ValidatesDomain pins BE-C-15: the record path must run
// the domain closed-set validation instead of hardcoding a writable row.
func TestRecordTransaction_ValidatesDomain(t *testing.T) {
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "tx-1"}}
	_, err := svc.RecordTransaction(context.Background(), "org-1", "user-1", "", "", "not-a-type", 10, "USD", "", "", nil)
	if err == nil {
		t.Fatal("RecordTransaction(invalid type) error = nil, want domain validation rejection")
	}
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
}

func refundGuardService(t *testing.T, original *domain.Transaction, refunds []*domain.Transaction) Service {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)
	mockUOW.EXPECT().Transactions().Return(mockTxRepo).AnyTimes()
	mockTxRepo.EXPECT().GetByIDForOrganization(gomock.Any(), original.ID, "org-1").Return(original, nil).AnyTimes()
	mockTxRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(refunds, int64(len(refunds)), nil).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	).AnyTimes()
	mockTxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	return Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "tx-refund"}, UOW: mockUOW}
}

func completedPayment(id string, amount float64) *domain.Transaction {
	return &domain.Transaction{
		ID: id, OrganizationID: "org-1", UserID: "user-1", OrderID: "order-1", PaymentID: "pay-1",
		TransactionType: "payment", Amount: amount, Status: "completed", Currency: "USD",
	}
}

// TestRefundTransaction_Guards pins BE-C-15: positive amount, cumulative cap
// per payment, and organization scoping before any refund row is written.
func TestRefundTransaction_Guards(t *testing.T) {
	tests := []struct {
		name     string
		original *domain.Transaction
		refunds  []*domain.Transaction
		amount   float64
		wantCode apperrors.Code
	}{
		{
			name: "non-positive amount", original: completedPayment("tx-1", 100), amount: 0,
			wantCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "single refund over original", original: completedPayment("tx-1", 100), amount: 101,
			wantCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "cumulative refunds over original", original: completedPayment("tx-1", 100),
			refunds: []*domain.Transaction{{
				ID: "tx-r1", OrganizationID: "org-1", OrderID: "order-1", PaymentID: "pay-1",
				TransactionType: "refund", Amount: 60, Status: "completed",
			}},
			amount: 50, wantCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "refunds of another payment do not count", original: completedPayment("tx-1", 100),
			refunds: []*domain.Transaction{{
				ID: "tx-r1", OrganizationID: "org-1", OrderID: "order-1", PaymentID: "pay-other",
				TransactionType: "refund", Amount: 90, Status: "completed",
			}},
			amount: 100, wantCode: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := refundGuardService(t, tt.original, tt.refunds)
			if tt.wantCode == "" {
				// Guards must not fire: the refund reaches the write path.
				result, err := svc.RefundTransaction(context.Background(), "org-1", tt.original.ID, tt.amount, nil)
				if err != nil {
					t.Fatalf("RefundTransaction() error = %v, want the guards to pass", err)
				}
				assert.Equal(t, "refund", result.TransactionType)
				return
			}
			_, err := svc.RefundTransaction(context.Background(), "org-1", tt.original.ID, tt.amount, nil)
			if err == nil {
				t.Fatal("RefundTransaction() error = nil, want guard rejection")
			}
			assert.Equal(t, tt.wantCode, apperrors.FromError(err).Code)
		})
	}
}

// TestRefundTransaction_RequiresOrganization pins the tenant assertion: the
// original transaction is read through the organization-scoped lookup, so a
// foreign id is reported missing before any refund is written.
func TestRefundTransaction_RequiresOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockTxRepo := repoMock.NewMockRepository(ctrl)
	mockUOW.EXPECT().Transactions().Return(mockTxRepo).AnyTimes()
	mockTxRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "tx-foreign", "org-1").
		Return(nil, apperrors.NewNotFound("transaction not found"))

	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "tx-refund"}, UOW: mockUOW}
	_, err := svc.RefundTransaction(context.Background(), "org-1", "tx-foreign", 10, nil)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)

	_, err = svc.RefundTransaction(context.Background(), "", "tx-foreign", 10, nil)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}
