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
	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ─── fakes ────────────────────────────────────────────────────────────────

type fakeClock struct{ fixedTime time.Time }

func (f fakeClock) Now() time.Time { return f.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (f fakeUUIDGenerator) NewString() (string, error) { return f.fixedID, nil }

// ─── Service.CreateNotification ───────────────────────────────────────────

func TestService_CreateNotification(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := nmock.NewMockRepository(ctrl)

	uow.EXPECT().Notifications().Return(repo).AnyTimes()
	npRepo := npmock.NewMockRepository(ctrl)
	uow.EXPECT().NotificationProviders().Return(npRepo).AnyTimes()
	npRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "provider-1", "org-1").
		Return(&providersdomain.NotificationProvider{ID: "provider-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "n-001"},
		UOW:   uow,
	}

	result, err := svc.CreateNotification(context.Background(), CreateNotificationCommand{
		OrganizationID: "org-1",
		ProviderID:     "provider-1",
		Recipient:      "user@example.com",
		Subject:        "Welcome",
		Content:        "Hello!",
	})
	require.NoError(t, err)
	assert.Equal(t, "n-001", result.ID)
	assert.Equal(t, "org-1", result.OrganizationID)
	assert.Equal(t, "pending", result.Status)
	assert.Equal(t, "user@example.com", result.Recipient)
	assert.Equal(t, "Welcome", result.Subject)
}

func TestService_CreateNotification_WithTemplate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := nmock.NewMockRepository(ctrl)

	uow.EXPECT().Notifications().Return(repo).AnyTimes()
	npRepo := npmock.NewMockRepository(ctrl)
	uow.EXPECT().NotificationProviders().Return(npRepo).AnyTimes()
	npRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "provider-1", "org-1").
		Return(&providersdomain.NotificationProvider{ID: "provider-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, n *domain.Notification) error {
			assert.Equal(t, "welcome-template", n.TemplateID)
			assert.Equal(t, map[string]any{"name": "Alice"}, n.Variables)
			return nil
		},
	)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "n-002"},
		UOW:   uow,
	}

	result, err := svc.CreateNotification(context.Background(), CreateNotificationCommand{
		OrganizationID: "org-1",
		ProviderID:     "provider-1",
		Recipient:      "user@example.com",
		Subject:        "Welcome",
		Content:        "Hello!",
		TemplateID:     "welcome-template",
		Variables:      map[string]any{"name": "Alice"},
	})
	require.NoError(t, err)
	assert.Equal(t, "n-002", result.ID)
}

// ─── Service.ListNotifications ────────────────────────────────────────────

func TestService_ListNotifications(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := nmock.NewMockRepository(ctrl)

	uow.EXPECT().Notifications().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), "org-1", "pending", "reset", 1, 20).Return(
		[]*domain.Notification{
			{ID: "n1", OrganizationID: "org-1", Status: "pending", Recipient: "a@b.com", CreatedAt: now, UpdatedAt: now},
			{ID: "n2", OrganizationID: "org-1", Status: "pending", Recipient: "c@d.com", CreatedAt: now, UpdatedAt: now},
		}, int64(2), nil,
	)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	items, total, err := svc.ListNotifications(context.Background(), ListNotificationsQuery{
		OrganizationID: "org-1",
		Status:         "pending",
		Keyword:        "reset",
		Page:           1,
		PageSize:       20,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
	assert.Equal(t, "n1", items[0].ID)
	assert.Equal(t, "n2", items[1].ID)
}

func TestService_ListNotifications_Empty(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := nmock.NewMockRepository(ctrl)

	uow.EXPECT().Notifications().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), "org-1", "", "", 1, 10).Return([]*domain.Notification{}, int64(0), nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	items, total, err := svc.ListNotifications(context.Background(), ListNotificationsQuery{
		OrganizationID: "org-1",
		Page:           1,
		PageSize:       10,
	})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

// ─── Service.MarkNotificationRead ─────────────────────────────────────────

func TestService_MarkNotificationRead(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := nmock.NewMockRepository(ctrl)

	uow.EXPECT().Notifications().Return(repo).AnyTimes()
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "n1", "org-1").
		Return(&domain.Notification{ID: "n1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().MarkSentForOrganization(gomock.Any(), "n1", "org-1").Return(nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.MarkNotificationRead(context.Background(), "org-1", "n1")
	require.NoError(t, err)
}

func TestService_MarkNotificationRead_NotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := nmock.NewMockRepository(ctrl)

	uow.EXPECT().Notifications().Return(repo).AnyTimes()
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "nonexistent", "org-1").
		Return(nil, apperrors.NewNotFound("notification not found"))

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.MarkNotificationRead(context.Background(), "org-1", "nonexistent")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestService_MarkNotificationRead_EmptyOrganizationRejected(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.MarkNotificationRead(context.Background(), "", "n1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

// ─── Service.MarkAllNotificationsRead ─────────────────────────────────────

func TestService_MarkAllNotificationsRead(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := nmock.NewMockRepository(ctrl)

	uow.EXPECT().Notifications().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), "org-1", "pending", "", 1, platformpagination.MaxPageSize).Return(
		[]*domain.Notification{
			{ID: "n1", OrganizationID: "org-1", Status: "pending"},
			{ID: "n2", OrganizationID: "org-1", Status: "pending"},
		}, int64(2), nil,
	)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, domainuow.UnitOfWork) error) error {
			txUOW := uowmock.NewMockUnitOfWork(ctrl)
			txRepo := nmock.NewMockRepository(ctrl)
			txUOW.EXPECT().Notifications().Return(txRepo).AnyTimes()
			txRepo.EXPECT().MarkSent(gomock.Any(), "n1").Return(nil)
			txRepo.EXPECT().MarkSent(gomock.Any(), "n2").Return(nil)
			return fn(ctx, txUOW)
		},
	)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.MarkAllNotificationsRead(context.Background(), "org-1")
	require.NoError(t, err)
}

func TestService_MarkAllNotificationsRead_Empty(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := nmock.NewMockRepository(ctrl)

	uow.EXPECT().Notifications().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), "org-1", "pending", "", 1, platformpagination.MaxPageSize).Return([]*domain.Notification{}, int64(0), nil)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test"},
		UOW:   uow,
	}

	err := svc.MarkAllNotificationsRead(context.Background(), "org-1")
	require.NoError(t, err)
}

// ─── NewService ───────────────────────────────────────────────────────────

func TestNewService(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uow := uowmock.NewMockUnitOfWork(ctrl)

	svc := NewService(fakeClock{fixedTime: time.Now()}, fakeUUIDGenerator{fixedID: "test"}, uow)
	assert.NotNil(t, svc.Clock)
	assert.NotNil(t, svc.UUID)
	assert.Equal(t, uow, svc.UOW)
}

// ─── DTO tests ────────────────────────────────────────────────────────────

func TestNotificationDTO_Fields(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	dto := NotificationDTO{
		ID:             "n1",
		OrganizationID: "org-1",
		ProviderID:     "provider-1",
		Recipient:      "user@example.com",
		Subject:        "Test",
		Content:        "Hello",
		Status:         "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	assert.Equal(t, "n1", dto.ID)
	assert.Equal(t, "org-1", dto.OrganizationID)
	assert.Equal(t, "pending", dto.Status)
}

func TestToNotificationDTO_Nil(t *testing.T) {
	t.Parallel()
	result := toNotificationDTO(nil)
	assert.Nil(t, result)
}

func TestToNotificationDTO_Success(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	n := &domain.Notification{
		ID:             "n1",
		OrganizationID: "org-1",
		ProviderID:     "provider-1",
		Recipient:      "user@example.com",
		Subject:        "Test",
		Content:        "Hello",
		Status:         "sent",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	dto := toNotificationDTO(n)
	assert.Equal(t, "n1", dto.ID)
	assert.Equal(t, "sent", dto.Status)
	assert.Equal(t, "user@example.com", dto.Recipient)
}

func TestCreateNotificationCommand_Fields(t *testing.T) {
	t.Parallel()
	cmd := CreateNotificationCommand{
		OrganizationID: "org-1",
		ProviderID:     "provider-1",
		Recipient:      "user@example.com",
		Subject:        "Welcome",
		Content:        "Hello, World!",
		TemplateID:     "welcome-template",
	}
	assert.Equal(t, "org-1", cmd.OrganizationID)
	assert.Equal(t, "user@example.com", cmd.Recipient)
}

func TestListNotificationsQuery_Fields(t *testing.T) {
	t.Parallel()
	q := ListNotificationsQuery{
		OrganizationID: "org-1",
		Status:         "pending",
		Page:           1,
		PageSize:       20,
	}
	assert.Equal(t, "org-1", q.OrganizationID)
	assert.Equal(t, "pending", q.Status)
	assert.Equal(t, 1, q.Page)
	assert.Equal(t, 20, q.PageSize)
}
