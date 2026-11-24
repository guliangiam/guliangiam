package usernotifications

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) List(ctx context.Context, organizationID, userID string, filter domain.ListFilter) ([]*domain.Notification, int64, error) {
	offset := shared.PageOffset(filter.Page, filter.PageSize)
	rows, err := r.db.Query(ctx, queryListNotifications,
		organizationID, userID, filter.Category, filter.IsRead, filter.Keyword,
		shared.ClampPageSize(filter.PageSize), offset)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "list user notifications")
	}
	defer rows.Close()

	items := make([]*domain.Notification, 0)
	for rows.Next() {
		item, scanErr := scanNotification(rows)
		if scanErr != nil {
			return nil, 0, apperrors.WrapInternalf(scanErr, "scan user notification")
		}
		items = append(items, item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, apperrors.WrapInternalf(rowsErr, "iterate user notifications")
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountNotifications,
		organizationID, userID, filter.Category, filter.IsRead, filter.Keyword).Scan(&total); err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "count user notifications")
	}
	return items, total, nil
}

func (r Repository) GetForUser(ctx context.Context, id, organizationID, userID string) (*domain.Notification, error) {
	row := r.db.QueryRow(ctx, queryGetNotificationForUser, id, organizationID, userID)
	item, err := scanNotification(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("notification not found")
		}
		return nil, apperrors.WrapInternalf(err, "get user notification")
	}
	return item, nil
}

func (r Repository) UnreadCount(ctx context.Context, organizationID, userID string) (int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, queryUnreadCount, organizationID, userID).Scan(&total); err != nil {
		return 0, apperrors.WrapInternalf(err, "count unread notifications")
	}
	return total, nil
}

func (r Repository) Create(ctx context.Context, notification *domain.Notification) error {
	_, err := r.db.Exec(ctx, queryInsertNotification,
		notification.ID, notification.OrganizationID, notification.UserID,
		notification.Category, notification.EventType, notification.Title,
		notification.Content, notification.Link, notification.SourceType,
		nullableString(notification.SourceID), notification.AllowReply,
		notification.IsRead, notification.ReadAt, notification.CreatedAt,
		notification.UpdatedAt)
	if err != nil {
		return apperrors.WrapInternalf(err, "create user notification")
	}
	return nil
}

func (r Repository) ExistsBySource(ctx context.Context, organizationID, userID, sourceType, sourceID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, queryExistsBySource, organizationID, userID, sourceType, sourceID).Scan(&exists); err != nil {
		return false, apperrors.WrapInternalf(err, "check user notification source")
	}
	return exists, nil
}

func (r Repository) MarkRead(ctx context.Context, id, organizationID, userID string, read bool, at time.Time) error {
	tag, err := r.db.Exec(ctx, queryMarkRead, id, organizationID, userID, read, at)
	if err != nil {
		return apperrors.WrapInternalf(err, "mark user notification read")
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("notification not found")
	}
	return nil
}

func (r Repository) MarkAllRead(ctx context.Context, organizationID, userID, category string, at time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, queryMarkAllRead, organizationID, userID, category, at)
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "mark all user notifications read")
	}
	return tag.RowsAffected(), nil
}

func (r Repository) SoftDelete(ctx context.Context, id, organizationID, userID string, at time.Time) error {
	tag, err := r.db.Exec(ctx, querySoftDelete, id, organizationID, userID, at)
	if err != nil {
		return apperrors.WrapInternalf(err, "delete user notification")
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("notification not found")
	}
	return nil
}

func (r Repository) BatchSoftDelete(ctx context.Context, ids []string, organizationID, userID string, at time.Time) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := r.db.Exec(ctx, queryBatchSoftDelete, ids, organizationID, userID, at)
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "batch delete user notifications")
	}
	return tag.RowsAffected(), nil
}

func (r Repository) ListReplies(ctx context.Context, notificationID, organizationID, userID string) ([]*domain.Reply, error) {
	rows, err := r.db.Query(ctx, queryListReplies, notificationID, organizationID, userID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list notification replies")
	}
	defer rows.Close()

	items := make([]*domain.Reply, 0)
	for rows.Next() {
		item := &domain.Reply{}
		if err := rows.Scan(&item.ID, &item.NotificationID, &item.OrganizationID,
			&item.UserID, &item.Content, &item.CreatedAt, &item.UpdatedAt, &item.Revision); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan notification reply")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "iterate notification replies")
	}
	return items, nil
}

func (r Repository) CreateReply(ctx context.Context, reply *domain.Reply) error {
	_, err := r.db.Exec(ctx, queryInsertReply,
		reply.ID, reply.NotificationID, reply.OrganizationID, reply.UserID,
		reply.Content, reply.CreatedAt, reply.UpdatedAt)
	if err != nil {
		return apperrors.WrapInternalf(err, "create notification reply")
	}
	return nil
}

func (r Repository) ListPreferences(ctx context.Context, organizationID, userID string) ([]*domain.Preference, error) {
	rows, err := r.db.Query(ctx, queryListPreferences, organizationID, userID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list notification preferences")
	}
	defer rows.Close()

	items := make([]*domain.Preference, 0)
	for rows.Next() {
		item := &domain.Preference{}
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.UserID, &item.Category,
			&item.Channel, &item.Enabled, &item.CreatedAt, &item.UpdatedAt, &item.Revision); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan notification preference")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "iterate notification preferences")
	}
	return items, nil
}

func (r Repository) UpsertPreferences(ctx context.Context, organizationID, userID string, prefs []*domain.Preference) error {
	for _, pref := range prefs {
		if _, err := r.db.Exec(ctx, queryUpsertPreference,
			pref.ID, organizationID, userID, pref.Category, pref.Channel,
			pref.Enabled, pref.CreatedAt, pref.UpdatedAt); err != nil {
			return apperrors.WrapInternalf(err, "upsert notification preference")
		}
	}
	return nil
}

func (r Repository) ListChannelBindings(ctx context.Context, organizationID, userID string) ([]*domain.ChannelBinding, error) {
	rows, err := r.db.Query(ctx, queryListChannelBindings, organizationID, userID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list notification channel bindings")
	}
	defer rows.Close()

	items := make([]*domain.ChannelBinding, 0)
	for rows.Next() {
		item := &domain.ChannelBinding{}
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.UserID, &item.Channel,
			&item.ExternalID, &item.Verified, &item.CreatedAt, &item.UpdatedAt, &item.Revision); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan notification channel binding")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "iterate notification channel bindings")
	}
	return items, nil
}

func (r Repository) UpsertChannelBinding(ctx context.Context, binding *domain.ChannelBinding) error {
	_, err := r.db.Exec(ctx, queryUpsertChannelBinding,
		binding.ID, binding.OrganizationID, binding.UserID, binding.Channel,
		binding.ExternalID, binding.Verified, binding.CreatedAt, binding.UpdatedAt)
	if err != nil {
		return apperrors.WrapInternalf(err, "upsert notification channel binding")
	}
	return nil
}

func (r Repository) DeleteChannelBinding(ctx context.Context, id, organizationID, userID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteChannelBinding, id, organizationID, userID)
	if err != nil {
		return apperrors.WrapInternalf(err, "delete notification channel binding")
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("notification channel binding not found")
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNotification(row rowScanner) (*domain.Notification, error) {
	item := &domain.Notification{}
	if err := row.Scan(&item.ID, &item.OrganizationID, &item.UserID, &item.Category,
		&item.EventType, &item.Title, &item.Content, &item.Link, &item.SourceType,
		&item.SourceID, &item.AllowReply, &item.IsRead, &item.ReadAt, &item.DeletedAt,
		&item.CreatedAt, &item.UpdatedAt, &item.Revision); err != nil {
		return nil, err
	}
	return item, nil
}

func nullableString(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}
