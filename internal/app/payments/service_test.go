package payments

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	repoMock "github.com/guliangiam/guliangiam/internal/domain/payments/mock"
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

func TestCreatePayment_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "p-001"},
		UOW:   mockUOW,
	}

	t.Run("missing_orgId", func(t *testing.T) {
		err := svc.CreatePayment(context.Background(), CreatePaymentCommand{Name: "x"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("missing_name", func(t *testing.T) {
		err := svc.CreatePayment(context.Background(), CreatePaymentCommand{OrganizationID: "org-1"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestCreatePayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockPaymentsRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Payments().Return(mockPaymentsRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockPaymentsRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "p-001"},
		UOW:   mockUOW,
	}

	err := svc.CreatePayment(context.Background(), CreatePaymentCommand{
		OrganizationID: "org-1",
		Name:           "payment-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListPayments_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockPaymentsRepo := repoMock.NewMockRepository(ctrl)

	now := time.Now()
	payments := []*domain.Payment{
		{ID: "p-1", Name: "payment-1", State: domain.StateCreated, CreatedAt: now, UpdatedAt: now},
		{ID: "p-2", Name: "payment-2", State: domain.StateSucceeded, CreatedAt: now, UpdatedAt: now},
	}

	mockUOW.EXPECT().Payments().Return(mockPaymentsRepo).AnyTimes()
	mockPaymentsRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(payments, int64(2), nil)

	svc := Service{UOW: mockUOW}

	items, total, err := svc.ListPayments(context.Background(), ListPaymentsQuery{
		Page:           1,
		PageSize:       50,
		OrganizationID: "org-001",
	})
	if err != nil {
		t.Fatalf("ListPayments() returned unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want %d", total, 2)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want %d", len(items), 2)
	}
}

func TestGetPayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockPaymentsRepo := repoMock.NewMockRepository(ctrl)

	now := time.Now()
	payment := &domain.Payment{ID: "p-1", OrganizationID: "org-1", Name: "payment-1", State: domain.StateSucceeded, CreatedAt: now, UpdatedAt: now}

	mockUOW.EXPECT().Payments().Return(mockPaymentsRepo).AnyTimes()
	mockPaymentsRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "p-1", "org-1").Return(payment, nil)

	svc := Service{UOW: mockUOW}

	detail, err := svc.GetPayment(context.Background(), "org-1", "p-1")
	if err != nil {
		t.Fatalf("GetPayment() returned unexpected error: %v", err)
	}
	if detail == nil {
		t.Fatal("expected non-nil detail")
	}
	if detail.ID != "p-1" {
		t.Errorf("detail.ID = %q, want %q", detail.ID, "p-1")
	}
}

func TestGetPayment_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockPaymentsRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Payments().Return(mockPaymentsRepo).AnyTimes()
	mockPaymentsRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "nonexistent", "org-1").Return(nil, apperrors.NewNotFound("payment not found"))

	svc := Service{UOW: mockUOW}

	detail, err := svc.GetPayment(context.Background(), "org-1", "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if detail != nil {
		t.Errorf("expected nil detail, got %+v", detail)
	}
}
