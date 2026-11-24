package payments

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	repoMock "github.com/guliangiam/guliangiam/internal/domain/payments/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPatchPayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockRepo := repoMock.NewMockRepository(ctrl)
	now := time.Now()

	mockUOW.EXPECT().Payments().Return(mockRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "p-1", "org-1").Return(&domain.Payment{
		ID: "p-1", OrganizationID: "org-1", State: domain.StateCreated, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}, nil)
	mockRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: mockUOW}
	displayName := "Updated Payment"
	state := domain.StateSucceeded
	err := svc.PatchPayment(context.Background(), "org-1", PatchPaymentCommand{
		ID:          "p-1",
		DisplayName: &displayName,
		State:       &state,
		Revision:    1,
	})
	require.NoError(t, err)
}

func TestPatchPayment_RevisionMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockRepo := repoMock.NewMockRepository(ctrl)
	now := time.Now()

	mockUOW.EXPECT().Payments().Return(mockRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "p-1", "org-1").Return(&domain.Payment{
		ID: "p-1", OrganizationID: "org-1", State: domain.StateCreated, Revision: 2, CreatedAt: now, UpdatedAt: now,
	}, nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: mockUOW}
	err := svc.PatchPayment(context.Background(), "org-1", PatchPaymentCommand{
		ID:       "p-1",
		Revision: 1,
	})
	assert.Error(t, err)
}

func TestDeletePayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Payments().Return(mockRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "p-1", "org-1").Return(&domain.Payment{
		ID: "p-1", OrganizationID: "org-1", User: "user-1",
	}, nil)
	mockRepo.EXPECT().Delete(gomock.Any(), "p-1", "org-1").Return(nil)

	svc := Service{UOW: mockUOW}
	err := svc.DeletePayment(context.Background(), "org-1", "p-1")
	require.NoError(t, err)
}

func TestInvoicePayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockRepo := repoMock.NewMockRepository(ctrl)
	now := time.Now()

	mockUOW.EXPECT().Payments().Return(mockRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "p-1", "org-1").Return(&domain.Payment{
		ID: "p-1", OrganizationID: "org-1", State: domain.StateSucceeded, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}, nil)
	mockRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: mockUOW}
	err := svc.InvoicePayment(context.Background(), "org-1", InvoicePaymentCommand{
		ID:           "p-1",
		InvoiceType:  "electronic",
		InvoiceTitle: "Company Inc",
		PersonName:   "Zhang San",
		PersonEmail:  "test@example.com",
		Revision:     1,
	})
	require.NoError(t, err)
}

func TestGetUserPayments_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockRepo := repoMock.NewMockRepository(ctrl)
	now := time.Now()

	mockUOW.EXPECT().Payments().Return(mockRepo).AnyTimes()
	mockRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domain.Payment{
		{ID: "p-1", User: "user-1", State: domain.StateSucceeded, CreatedAt: now, UpdatedAt: now},
	}, int64(1), nil)

	svc := Service{UOW: mockUOW}
	items, total, err := svc.GetUserPayments(context.Background(), "org-1", "user-1", "user-1", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
}

func TestGetPaymentResult_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockRepo := repoMock.NewMockRepository(ctrl)
	now := time.Now()

	mockUOW.EXPECT().Payments().Return(mockRepo).AnyTimes()
	mockRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "p-1", "org-1").Return(&domain.Payment{
		ID: "p-1", OrganizationID: "org-1", State: domain.StateSucceeded, Price: 99.99, Currency: "CNY",
		ProductName: "VPN Plan", Provider: "alipay", PayUrl: "https://pay.url",
		InvoiceUrl: "https://inv.url", CreatedAt: now, UpdatedAt: now,
	}, nil)

	svc := Service{UOW: mockUOW}
	result, err := svc.GetPaymentResult(context.Background(), "org-1", "p-1")
	require.NoError(t, err)
	assert.Equal(t, "p-1", result.ID)
	assert.Equal(t, domain.StateSucceeded, result.State)
	assert.Equal(t, 99.99, result.Price)
	assert.Equal(t, "alipay", result.ProviderName)
}

func TestToListItem(t *testing.T) {
	now := time.Now()
	p := &domain.Payment{
		ID: "p-1", OrganizationID: "org-1", Name: "test", DisplayName: "Test",
		Provider: "alipay", Type: "recharge", User: "user-1",
		Price: 99.99, Currency: "CNY", State: domain.StateSucceeded,
		CreatedAt: now, UpdatedAt: now,
	}
	item := toListItem(p)
	assert.Equal(t, "p-1", item.ID)
	assert.Equal(t, "org-1", item.OrganizationID)
	assert.Equal(t, 99.99, item.Price)
}

func TestToDetail(t *testing.T) {
	now := time.Now()
	p := &domain.Payment{
		ID: "p-1", OrganizationID: "org-1", Name: "test", DisplayName: "Test",
		Products: []string{"prod-1"}, ProductName: "Product 1",
		PersonName: "Zhang San", State: domain.StateSucceeded, Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	detail := toDetail(p)
	assert.Equal(t, "p-1", detail.ID)
	assert.Equal(t, []string{"prod-1"}, detail.Products)
	assert.Equal(t, "Zhang San", detail.PersonName)
	assert.Equal(t, int64(1), detail.Revision)
}
