// Package usernotifications provides the MySQL mirror for the per-user inbox.
package usernotifications

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Repository struct {
	db mysqlshared.DBTX
}

func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

const notificationColumns = `id, organization_id, user_id, category, event_type, title, content, link,
	source_type, source_id, allow_reply, is_read, read_at, deleted_at, created_at, updated_at, revision`

func (r Repository) List(ctx context.Context, organizationID, userID string, filter domain.ListFilter) ([]*domain.Notification, int64, error) {
	offset := pageOffset(filter.Page, filter.PageSize)
	isRead := nullableBool(filter.IsRead)
	baseWhere := `
FROM user_notifications
WHERE organization_id = ?
  AND user_id = ?
  AND deleted_at IS NULL
  AND (? = '' OR category = ?)
  AND (? IS NULL OR is_read = ?)
  AND (? = '' OR title LIKE CONCAT('%', ?, '%') OR content LIKE CONCAT('%', ?, '%'))`

	args := []any{
		organizationID, userID,
		filter.Category, filter.Category,
		isRead, isRead,
		filter.Keyword, filter.Keyword, filter.Keyword,
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT "+notificationColumns+baseWhere+" ORDER BY created_at DESC LIMIT ? OFFSET ?",
		append(args, clampPageSize(filter.PageSize), offset)...)
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
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*)"+baseWhere, args...).Scan(&total); err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "count user notifications")
	}
	return items, total, nil
}

func (r Repository) GetForUser(ctx context.Context, id, organizationID, userID string) (*domain.Notification, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+notificationColumns+` FROM user_notifications
		 WHERE id = ? AND organization_id = ? AND user_id = ? AND deleted_at IS NULL`,
		id, organizationID, userID)
	item, err := scanNotification(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("notification not found")
		}
		return nil, apperrors.WrapInternalf(err, "get user notification")
	}
	return item, nil
}

func (r Repository) UnreadCount(ctx context.Context, organizationID, userID string) (int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_notifications
		 WHERE organization_id = ? AND user_id = ? AND is_read = 0 AND deleted_at IS NULL`,
		organizationID, userID).Scan(&total); err != nil {
		return 0, apperrors.WrapInternalf(err, "count unread notifications")
	}
	return total, nil
}

func (r Repository) ExistsBySource(ctx context.Context, organizationID, userID, sourceType, sourceID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM user_notifications
			WHERE organization_id = ? AND user_id = ? AND source_type = ? AND source_id = ?
			  AND deleted_at IS NULL
		)`, organizationID, userID, sourceType, sourceID).Scan(&exists)
	if err != nil {
		return false, apperrors.WrapInternalf(err, "check user notification source")
	}
	return exists, nil
}

func (r Repository) Create(ctx context.Context, notification *domain.Notification) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_notifications (
			id, organization_id, user_id, category, event_type, title, content, link,
			source_type, source_id, allow_reply, is_read, read_at, created_at, updated_at, revision
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
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

func (r Repository) MarkRead(ctx context.Context, id, organizationID, userID string, read bool, at time.Time) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE user_notifications
		 SET is_read = ?, read_at = CASE WHEN ? THEN COALESCE(read_at, ?) ELSE NULL END,
		     updated_at = ?, revision = revision + 1
		 WHERE id = ? AND organization_id = ? AND user_id = ? AND deleted_at IS NULL`,
		read, read, at, at, id, organizationID, userID)
	if err != nil {
		return apperrors.WrapInternalf(err, "mark user notification read")
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return apperrors.NewNotFound("notification not found")
	}
	return nil
}

func (r Repository) MarkAllRead(ctx context.Context, organizationID, userID, category string, at time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE user_notifications
		 SET is_read = 1, read_at = COALESCE(read_at, ?), updated_at = ?, revision = revision + 1
		 WHERE organization_id = ? AND user_id = ? AND is_read = 0 AND deleted_at IS NULL
		   AND (? = '' OR category = ?)`,
		at, at, organizationID, userID, category, category)
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "mark all user notifications read")
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (r Repository) SoftDelete(ctx context.Context, id, organizationID, userID string, at time.Time) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE user_notifications SET deleted_at = ?, updated_at = ?, revision = revision + 1
		 WHERE id = ? AND organization_id = ? AND user_id = ? AND deleted_at IS NULL`,
		at, at, id, organizationID, userID)
	if err != nil {
		return apperrors.WrapInternalf(err, "delete user notification")
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return apperrors.NewNotFound("notification not found")
	}
	return nil
}

func (r Repository) BatchSoftDelete(ctx context.Context, ids []string, organizationID, userID string, at time.Time) (int64, error) {
	var total int64
	for _, id := range ids {
		result, err := r.db.ExecContext(ctx,
			`UPDATE user_notifications SET deleted_at = ?, updated_at = ?, revision = revision + 1
			 WHERE id = ? AND organization_id = ? AND user_id = ? AND deleted_at IS NULL`,
			at, at, id, organizationID, userID)
		if err != nil {
			return total, apperrors.WrapInternalf(err, "batch delete user notifications")
		}
		affected, _ := result.RowsAffected()
		total += affected
	}
	return total, nil
}

func (r Repository) ListReplies(ctx context.Context, notificationID, organizationID, userID string) ([]*domain.Reply, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, notification_id, organization_id, user_id, content, created_at, updated_at, revision
		 FROM user_notification_replies
		 WHERE notification_id = ? AND organization_id = ? AND user_id = ?
		 ORDER BY created_at ASC`, notificationID, organizationID, userID)
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
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_notification_replies (
			id, notification_id, organization_id, user_id, content, created_at, updated_at, revision
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, 1)`,
		reply.ID, reply.NotificationID, reply.OrganizationID, reply.UserID,
		reply.Content, reply.CreatedAt, reply.UpdatedAt)
	if err != nil {
		return apperrors.WrapInternalf(err, "create notification reply")
	}
	return nil
}

func (r Repository) ListPreferences(ctx context.Context, organizationID, userID string) ([]*domain.Preference, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, organization_id, user_id, category, channel, enabled, created_at, updated_at, revision
		 FROM notification_preferences
		 WHERE organization_id = ? AND user_id = ?
		 ORDER BY category, channel`, organizationID, userID)
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
		if _, err := r.db.ExecContext(ctx,
			`INSERT INTO notification_preferences (
				id, organization_id, user_id, category, channel, enabled, created_at, updated_at, revision
			 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
			 ON DUPLICATE KEY UPDATE enabled = VALUES(enabled), updated_at = VALUES(updated_at), revision = revision + 1`,
			pref.ID, organizationID, userID, pref.Category, pref.Channel,
			pref.Enabled, pref.CreatedAt, pref.UpdatedAt); err != nil {
			return apperrors.WrapInternalf(err, "upsert notification preference")
		}
	}
	return nil
}

func (r Repository) ListChannelBindings(ctx context.Context, organizationID, userID string) ([]*domain.ChannelBinding, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, organization_id, user_id, channel, external_id, verified, created_at, updated_at, revision
		 FROM notification_channel_bindings
		 WHERE organization_id = ? AND user_id = ?
		 ORDER BY channel, created_at`, organizationID, userID)
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
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO notification_channel_bindings (
			id, organization_id, user_id, channel, external_id, verified, created_at, updated_at, revision
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
		 ON DUPLICATE KEY UPDATE verified = VALUES(verified), updated_at = VALUES(updated_at), revision = revision + 1`,
		binding.ID, binding.OrganizationID, binding.UserID, binding.Channel,
		binding.ExternalID, binding.Verified, binding.CreatedAt, binding.UpdatedAt)
	if err != nil {
		return apperrors.WrapInternalf(err, "upsert notification channel binding")
	}
	return nil
}

func (r Repository) DeleteChannelBinding(ctx context.Context, id, organizationID, userID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM notification_channel_bindings WHERE id = ? AND organization_id = ? AND user_id = ?`,
		id, organizationID, userID)
	if err != nil {
		return apperrors.WrapInternalf(err, "delete notification channel binding")
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return apperrors.NewNotFound("notification channel binding not found")
	}
	return nil
}

func scanNotification(scanner interface{ Scan(...any) error }) (*domain.Notification, error) {
	item := &domain.Notification{}
	var sourceID sql.NullString
	if err := scanner.Scan(&item.ID, &item.OrganizationID, &item.UserID, &item.Category,
		&item.EventType, &item.Title, &item.Content, &item.Link, &item.SourceType,
		&sourceID, &item.AllowReply, &item.IsRead, &item.ReadAt, &item.DeletedAt,
		&item.CreatedAt, &item.UpdatedAt, &item.Revision); err != nil {
		return nil, err
	}
	if sourceID.Valid {
		item.SourceID = &sourceID.String
	}
	return item, nil
}

func nullableString(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}

func nullableBool(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

func clampPageSize(size int) int {
	if size <= 0 {
		return 20
	}
	if size > 100 {
		return 100
	}
	return size
}

func pageOffset(page, pageSize int) int {
	if page <= 1 {
		return 0
	}
	return (page - 1) * clampPageSize(pageSize)
}
