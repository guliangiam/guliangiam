package notifications

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	mockuow "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	providermock "github.com/guliangiam/guliangiam/internal/domain/notificationproviders/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	notifmock "github.com/guliangiam/guliangiam/internal/domain/notifications/mock"
)

func newTestWorker(uow appuow.UnitOfWork, factory domain.NotificationFactory) *Worker {
	w := &Worker{
		UOW:            uow,
		Factory:        factory,
		MaxRetries:     0,
		Timeout:        0,
		BaseRetryDelay: 0,
		BatchSize:      10,
		logger:         slog.Default(),
	}
	return w
}

func TestWorker_ProcessPendingNotifications_ListError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)

	uowMock.EXPECT().Notifications().Return(notifRepo)
	notifRepo.EXPECT().ListPendingRetries(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	w := newTestWorker(uowMock, nil)
	w.processPendingNotifications()
}

func TestWorker_ProcessPendingNotifications_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)

	uowMock.EXPECT().Notifications().Return(notifRepo)
	notifRepo.EXPECT().ListPendingRetries(gomock.Any(), gomock.Any()).Return([]*domain.Notification{}, nil)

	w := newTestWorker(uowMock, nil)
	w.processPendingNotifications()
}

func TestWorker_DeliverNotification_ProviderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	providerRepo := providermock.NewMockRepository(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(nil, assert.AnError)

	uowMock.EXPECT().Notifications().Return(notifRepo)
	notifRepo.EXPECT().MarkFailed(gomock.Any(), "notif1", gomock.Any()).Return(nil)

	w := newTestWorker(uowMock, nil)
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:         "notif1",
		ProviderID: "prov1",
	})
}

func TestWorker_DeliverNotification_DisabledProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	providerRepo := providermock.NewMockRepository(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(&providerdomain.NotificationProvider{
		ID:        "prov1",
		IsEnabled: false,
	}, nil)

	// BE-B-9: a disabled provider terminates the row instead of leaving it
	// pending forever at the head of the ASC retry queue.
	uowMock.EXPECT().Notifications().Return(notifRepo)
	notifRepo.EXPECT().MarkFailed(gomock.Any(), "notif1", "notification provider is disabled").Return(nil)

	w := newTestWorker(uowMock, nil)
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:         "notif1",
		ProviderID: "prov1",
	})
}

func TestWorker_DeliverNotification_NoProviderType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	providerRepo := providermock.NewMockRepository(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(&providerdomain.NotificationProvider{
		ID:           "prov1",
		IsEnabled:    true,
		ProviderType: "",
	}, nil)

	// BE-B-9: an untyped provider terminates the row (permanent config error).
	uowMock.EXPECT().Notifications().Return(notifRepo)
	notifRepo.EXPECT().MarkFailed(gomock.Any(), "notif1", "notification provider has no type").Return(nil)

	w := newTestWorker(uowMock, nil)
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:         "notif1",
		ProviderID: "prov1",
	})
}

func TestWorker_DeliverNotification_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	providerRepo := providermock.NewMockRepository(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)
	factory := notifmock.NewMockNotificationFactory(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(&providerdomain.NotificationProvider{
		ID:             "prov1",
		OrganizationID: "org1",
		IsEnabled:      true,
		ProviderType:   "email",
	}, nil)

	factory.EXPECT().SendNotification(gomock.Any(), "email", gomock.Any()).Return(nil)

	uowMock.EXPECT().Notifications().Return(notifRepo)
	notifRepo.EXPECT().MarkSent(gomock.Any(), "notif1").Return(nil)

	w := newTestWorker(uowMock, factory)
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:             "notif1",
		ProviderID:     "prov1",
		OrganizationID: "org1",
		Recipient:      "user@example.com",
		Subject:        "Test",
		Content:        "Hello",
	})
}

func TestWorker_DeliverNotification_SuccessWithWebhookURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	providerRepo := providermock.NewMockRepository(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)
	factory := notifmock.NewMockNotificationFactory(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(&providerdomain.NotificationProvider{
		ID:             "prov1",
		OrganizationID: "org1",
		IsEnabled:      true,
		ProviderType:   "webhook",
		WebhookURL:     "https://hooks.example.com",
		AccessToken:    "token123",
		Config:         map[string]any{"key": "value"},
	}, nil)

	factory.EXPECT().SendNotification(gomock.Any(), "webhook", gomock.Any()).Return(nil)

	uowMock.EXPECT().Notifications().Return(notifRepo)
	notifRepo.EXPECT().MarkSent(gomock.Any(), "notif1").Return(nil)

	w := newTestWorker(uowMock, factory)
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:             "notif1",
		ProviderID:     "prov1",
		OrganizationID: "org1",
		Recipient:      "user@example.com",
		Subject:        "Test",
		Content:        "Hello",
	})
}

func TestWorker_DeliverNotification_LogsMaskedRecipient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	providerRepo := providermock.NewMockRepository(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)
	factory := notifmock.NewMockNotificationFactory(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(&providerdomain.NotificationProvider{
		ID:             "prov1",
		OrganizationID: "org1",
		IsEnabled:      true,
		ProviderType:   "email",
	}, nil)

	factory.EXPECT().SendNotification(gomock.Any(), "email", gomock.Any()).Return(nil)

	uowMock.EXPECT().Notifications().Return(notifRepo)
	notifRepo.EXPECT().MarkSent(gomock.Any(), "notif1").Return(nil)

	var buf bytes.Buffer
	w := newTestWorker(uowMock, factory)
	w.logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:             "notif1",
		ProviderID:     "prov1",
		OrganizationID: "org1",
		Recipient:      "user@example.com",
		Subject:        "Test",
		Content:        "Hello",
	})

	out := buf.String()
	assert.NotContains(t, out, "user@example.com")
	assert.Contains(t, out, "u***@example.com")
}

func TestWorker_DeliverNotification_FailAfterRetries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	providerRepo := providermock.NewMockRepository(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)
	factory := notifmock.NewMockNotificationFactory(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(&providerdomain.NotificationProvider{
		ID:             "prov1",
		OrganizationID: "org1",
		IsEnabled:      true,
		ProviderType:   "email",
	}, nil)

	factory.EXPECT().SendNotification(gomock.Any(), "email", gomock.Any()).Return(assert.AnError)

	// MaxRetries=0 → the first failure exhausts the budget: the attempt is
	// persisted (IncrementRetryCount) and the row reaches terminal failed.
	uowMock.EXPECT().Notifications().Return(notifRepo).Times(2)
	notifRepo.EXPECT().IncrementRetryCount(gomock.Any(), "notif1").Return(nil)
	notifRepo.EXPECT().MarkFailed(gomock.Any(), "notif1", gomock.Any()).Return(nil)

	w := newTestWorker(uowMock, factory)
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:             "notif1",
		ProviderID:     "prov1",
		OrganizationID: "org1",
		Recipient:      "user@example.com",
		Subject:        "Test",
		Content:        "Hello",
	})
}

// BE-B-9 regression: a failed attempt with remaining retry budget must persist
// retry_count progress (single attempt, no in-process sleep) and must NOT
// finalize the row; the next polling cycle retries it.
func TestWorker_DeliverNotification_FailureWithBudget_IncrementsOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	providerRepo := providermock.NewMockRepository(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)
	factory := notifmock.NewMockNotificationFactory(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(&providerdomain.NotificationProvider{
		ID:             "prov1",
		OrganizationID: "org1",
		IsEnabled:      true,
		ProviderType:   "email",
	}, nil)

	factory.EXPECT().SendNotification(gomock.Any(), "email", gomock.Any()).Return(assert.AnError).Times(1)

	uowMock.EXPECT().Notifications().Return(notifRepo).Times(1)
	notifRepo.EXPECT().IncrementRetryCount(gomock.Any(), "notif1").Return(nil)

	w := newTestWorker(uowMock, factory)
	w.MaxRetries = 3
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:             "notif1",
		ProviderID:     "prov1",
		OrganizationID: "org1",
		Recipient:      "user@example.com",
		RetryCount:     0,
	})
}

// BE-B-9 regression: the attempt that spends the last unit of the retry budget
// (retry_count 2 → 3 with MaxRetries=3) must persist the increment AND reach
// terminal failed with a sanitized reason.
func TestWorker_DeliverNotification_LastAttempt_MarksFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	providerRepo := providermock.NewMockRepository(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)
	factory := notifmock.NewMockNotificationFactory(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(&providerdomain.NotificationProvider{
		ID:             "prov1",
		OrganizationID: "org1",
		IsEnabled:      true,
		ProviderType:   "email",
	}, nil)

	factory.EXPECT().SendNotification(gomock.Any(), "email", gomock.Any()).
		Return(assert.AnError).Times(1)

	uowMock.EXPECT().Notifications().Return(notifRepo).Times(2)
	notifRepo.EXPECT().IncrementRetryCount(gomock.Any(), "notif1").Return(nil)
	notifRepo.EXPECT().MarkFailed(gomock.Any(), "notif1", gomock.Any()).
		DoAndReturn(func(_ context.Context, id, reason string) error {
			assert.Contains(t, reason, "delivery failed after 3 attempts")
			return nil
		})

	w := newTestWorker(uowMock, factory)
	w.MaxRetries = 3
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:             "notif1",
		ProviderID:     "prov1",
		OrganizationID: "org1",
		Recipient:      "user@example.com",
		RetryCount:     2,
	})
}

func TestWorker_DeliverNotification_ProviderNotFound_MarkFailedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := mockuow.NewMockUnitOfWork(ctrl)
	providerRepo := providermock.NewMockRepository(ctrl)
	notifRepo := notifmock.NewMockRepository(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(nil, assert.AnError)

	uowMock.EXPECT().Notifications().Return(notifRepo)
	notifRepo.EXPECT().MarkFailed(gomock.Any(), "notif1", gomock.Any()).Return(assert.AnError)

	w := newTestWorker(uowMock, nil)
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:         "notif1",
		ProviderID: "prov1",
	})
}
