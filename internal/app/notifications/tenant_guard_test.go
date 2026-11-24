package notifications

import (
	"context"
	"testing"
	"time"

	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	providersdomain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	npmock "github.com/guliangiam/guliangiam/internal/domain/notificationproviders/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	nmock "github.com/guliangiam/guliangiam/internal/domain/notifications/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// BE-B-3: a providerId belonging to another organization must be rejected
// before any notification row is written.
func TestService_CreateNotification_CrossTenantProviderRejected(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	npRepo := npmock.NewMockRepository(ctrl)

	uow.EXPECT().NotificationProviders().Return(npRepo).AnyTimes()
	npRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "prov-other", "org-1").
		Return(nil, apperrors.NewNotFound("notification provider not found"))
	// No Notifications().Create expectation: gomock fails on any write.

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "n-x"},
		UOW:   uow,
	}

	_, err := svc.CreateNotification(context.Background(), CreateNotificationCommand{
		OrganizationID: "org-1",
		ProviderID:     "prov-other",
		Recipient:      "user@example.com",
		Subject:        "s",
		Content:        "c",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

// Defense in depth: even if the scoped lookup returned a foreign provider,
// the explicit organization comparison must reject it.
func TestService_CreateNotification_ProviderOrgMismatchRejected(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	npRepo := npmock.NewMockRepository(ctrl)

	uow.EXPECT().NotificationProviders().Return(npRepo).AnyTimes()
	npRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "prov-1", "org-1").
		Return(&providersdomain.NotificationProvider{ID: "prov-1", OrganizationID: "org-2"}, nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "n-x"},
		UOW:   uow,
	}

	_, err := svc.CreateNotification(context.Background(), CreateNotificationCommand{
		OrganizationID: "org-1",
		ProviderID:     "prov-1",
		Recipient:      "user@example.com",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestService_CreateNotification_MissingProviderID(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "n-x"},
		UOW:   uow,
	}

	_, err := svc.CreateNotification(context.Background(), CreateNotificationCommand{
		OrganizationID: "org-1",
		Recipient:      "user@example.com",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

type countingFactory struct{ calls int }

func (f *countingFactory) SendNotification(_ context.Context, _ string, _ *domain.SendNotification) error {
	f.calls++
	return nil
}

// BE-B-3 worker side: a notification whose provider belongs to another
// organization is marked failed and never delivered.
func TestWorker_DeliverNotification_ProviderOrgMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := uowmock.NewMockUnitOfWork(ctrl)
	providerRepo := npmock.NewMockRepository(ctrl)
	notifRepo := nmock.NewMockRepository(ctrl)

	uowMock.EXPECT().NotificationProviders().Return(providerRepo)
	providerRepo.EXPECT().GetByID(gomock.Any(), "prov1").Return(
		&providersdomain.NotificationProvider{
			ID:             "prov1",
			OrganizationID: "org-2",
			IsEnabled:      true,
			ProviderType:   "email",
		}, nil)

	uowMock.EXPECT().Notifications().Return(notifRepo)
	notifRepo.EXPECT().MarkFailed(gomock.Any(), "notif1", "provider organization mismatch").Return(nil)

	factory := &countingFactory{}
	w := newTestWorker(uowMock, factory)
	w.deliverNotification(context.Background(), &domain.Notification{
		ID:             "notif1",
		OrganizationID: "org-1",
		ProviderID:     "prov1",
	})
	assert.Zero(t, factory.calls, "mismatched provider must never be used for delivery")
}
