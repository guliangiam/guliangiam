package notifications

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	nmock "github.com/guliangiam/guliangiam/internal/domain/notifications/mock"
	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// The pending set must be walked page by page: the old single
// List(..., 1, 1000) call was clamped to MaxPageSize by the repository, so
// notifications 101..201 stayed unread after "mark all". The walk now uses
// MaxPageSize and every collected id is marked in the same transaction.
func TestService_MarkAllNotificationsRead_ScansBeyondTheFirstPage(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := nmock.NewMockRepository(ctrl)
	uow.EXPECT().Notifications().Return(repo).AnyTimes()

	pending := make([]*domain.Notification, 2*platformpagination.MaxPageSize+1)
	for i := range pending {
		pending[i] = &domain.Notification{
			ID:             fmt.Sprintf("n-%03d", i),
			OrganizationID: "org-1",
			Status:         "pending",
		}
	}
	total := int64(len(pending))

	repo.EXPECT().List(gomock.Any(), "org-1", "pending", "", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _, _, _ string, page, pageSize int) ([]*domain.Notification, int64, error) {
			// Mirror the repository: an oversized request is clamped, which is
			// what made the old call return only the first 100 rows.
			assert.Equal(t, platformpagination.MaxPageSize, pageSize)
			offset := platformpagination.Offset(page, pageSize)
			if offset >= len(pending) {
				return nil, total, nil
			}
			end := offset + pageSize
			if end > len(pending) {
				end = len(pending)
			}
			return pending[offset:end], total, nil
		}).Times(3)

	marked := make([]string, 0, len(pending))
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, domainuow.UnitOfWork) error) error {
			txUOW := uowmock.NewMockUnitOfWork(ctrl)
			txRepo := nmock.NewMockRepository(ctrl)
			txUOW.EXPECT().Notifications().Return(txRepo).AnyTimes()
			txRepo.EXPECT().MarkSent(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, id string) error {
					marked = append(marked, id)
					return nil
				}).Times(len(pending))
			return fn(ctx, txUOW)
		}).Times(1)

	svc := Service{UOW: uow}
	err := svc.MarkAllNotificationsRead(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, marked, len(pending))
	assert.Equal(t, "n-200", marked[len(marked)-1], "the notification beyond the first page must be marked")
}

// A failing mark aborts the single transaction: the error leaves the callback,
// so the UoW rolls the whole batch back instead of committing a prefix. The
// fourth row must never be attempted after the failure.
func TestService_MarkAllNotificationsRead_MarkFailureAbortsTransaction(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := nmock.NewMockRepository(ctrl)
	uow.EXPECT().Notifications().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), "org-1", "pending", "", 1, platformpagination.MaxPageSize).Return(
		[]*domain.Notification{
			{ID: "n-1", OrganizationID: "org-1", Status: "pending"},
			{ID: "n-2", OrganizationID: "org-1", Status: "pending"},
			{ID: "n-3", OrganizationID: "org-1", Status: "pending"},
		}, int64(3), nil,
	)

	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, domainuow.UnitOfWork) error) error {
			txUOW := uowmock.NewMockUnitOfWork(ctrl)
			txRepo := nmock.NewMockRepository(ctrl)
			txUOW.EXPECT().Notifications().Return(txRepo).AnyTimes()
			txRepo.EXPECT().MarkSent(gomock.Any(), "n-1").Return(nil)
			txRepo.EXPECT().MarkSent(gomock.Any(), "n-2").Return(assert.AnError)
			return fn(ctx, txUOW)
		}).Times(1)

	svc := Service{UOW: uow}
	err := svc.MarkAllNotificationsRead(context.Background(), "org-1")
	require.ErrorIs(t, err, assert.AnError)
}

// An empty organization is rejected before the list runs. The SQL list treats
// an empty organization as "no org filter", so reaching it would mark every
// tenant's pending notifications.
func TestService_MarkAllNotificationsRead_EmptyOrganizationRejected(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := Service{UOW: uowmock.NewMockUnitOfWork(ctrl)}
	err := svc.MarkAllNotificationsRead(context.Background(), "")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}
