package notifications

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	notificationdomain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// MarkNotificationRead marks a notification as sent (acknowledged/read).
func (s Service) MarkNotificationRead(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	// Settle ownership before the write: a foreign id and a missing id must be
	// indistinguishable to the caller.
	if _, err := s.UOW.Notifications().GetByIDForOrganization(ctx, id, organizationID); err != nil {
		return err
	}
	if err := s.UOW.Notifications().MarkSentForOrganization(ctx, id, organizationID); err != nil {
		return err
	}
	return nil
}

// MarkAllNotificationsRead marks all pending notifications as sent for an organization.
//
// The pending set is collected outside the transaction and then marked inside a
// single transaction:
//
//   - Collecting first keeps the page walk stable. Marking while paging would
//     shrink the `pending` filter under the walk, so the rows that shift from
//     page 2 to page 1 would be skipped.
//   - One transaction, not one per batch: a batch commit followed by a failure
//     leaves the earlier batches written while the caller receives an error,
//     and the retry cannot tell which rows were already processed. The cost is
//     a larger transaction, paid deliberately for all-or-nothing semantics.
//   - Retries are idempotent: MarkSent only sets status='sent' for the selected
//     ids, so re-running the whole call re-marks already-sent rows and finishes
//     the remaining ones.
//
// Notifications created after the collection are not part of this call; they
// remain pending and are handled by the next invocation.
func (s Service) MarkAllNotificationsRead(ctx context.Context, organizationID string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}

	items, err := platformpagination.CollectAll(func(page, pageSize int) ([]*notificationdomain.Notification, int64, error) {
		return s.UOW.Notifications().List(ctx, organizationID, "pending", "", page, pageSize)
	})
	if err != nil {
		return apperrors.WrapInternal(err, "list pending notifications")
	}
	if len(items) == 0 {
		return nil
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo := tx.Notifications()
		for _, item := range items {
			if err := repo.MarkSent(ctx, item.ID); err != nil {
				return apperrors.WrapInternalf(err, "mark notification %s read", item.ID)
			}
		}
		return nil
	})
}
