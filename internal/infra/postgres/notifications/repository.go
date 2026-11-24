package notifications

import (
	"context"
	"encoding/json"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	return r.getByID(ctx, queryGetNotificationByID, id)
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Notification, error) {
	return r.getByID(ctx, queryGetNotificationByIDForOrganization, id, organizationID)
}

func (r Repository) getByID(ctx context.Context, query string, args ...any) (*domain.Notification, error) {
	var row notificationRow
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.ProviderID,
		&row.UserNotificationID, &row.Channel, &row.ProviderKind,
		&row.Recipient,
		&row.Subject, &row.Content, &row.TemplateID, &row.Variables,
		&row.Status, &row.Error, &row.SentAt, &row.RetryCount,
		&row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("notification not found")
		}
		return nil, apperrors.WrapInternalf(err, "get notification by id")
	}
	n := toNotification(row)
	return &n, nil
}

func (r Repository) List(ctx context.Context, organizationID, status, keyword string, page, pageSize int) ([]*domain.Notification, int64, error) {
	offset := shared.PageOffset(page, pageSize)
	rows, err := r.db.Query(ctx, queryListNotifications, organizationID, status, keyword, shared.ClampPageSize(pageSize), offset)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "list notifications")
	}
	defer rows.Close()

	items := make([]*domain.Notification, 0)
	for rows.Next() {
		var row notificationRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.ProviderID,
			&row.UserNotificationID, &row.Channel, &row.ProviderKind,
			&row.Recipient,
			&row.Subject, &row.Content, &row.TemplateID, &row.Variables,
			&row.Status, &row.Error, &row.SentAt, &row.RetryCount,
			&row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "scan notification row")
		}
		n := toNotification(row)
		items = append(items, &n)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, apperrors.WrapInternalf(rowsErr, "iterate notifications")
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountNotifications, organizationID, status, keyword).Scan(&total); err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "count notifications")
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, notification *domain.Notification) error {
	variables, _ := json.Marshal(notification.Variables)
	_, err := r.db.Exec(ctx, queryInsertNotification,
		notification.ID, notification.OrganizationID, notification.ProviderID,
		notification.UserNotificationID, notification.Channel, notification.ProviderKind,
		notification.Recipient, nullable(notification.Subject), notification.Content,
		nullable(notification.TemplateID), variables, notification.Status,
		nullable(notification.Error), notification.SentAt, notification.RetryCount,
		notification.CreatedAt, notification.UpdatedAt,
	)
	return err
}

func (r Repository) Update(ctx context.Context, notification *domain.Notification) error {
	variables, _ := json.Marshal(notification.Variables)
	_, err := r.db.Exec(ctx, queryUpdateNotification,
		notification.ID, notification.ProviderID, notification.Recipient,
		notification.Subject, notification.Content, nullable(notification.TemplateID),
		variables, notification.Status, nullable(notification.Error),
		notification.SentAt, notification.RetryCount, notification.UpdatedAt,
	)
	return err
}

func (r Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, queryDeleteNotification, id)
	return err
}

func (r Repository) MarkSent(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx, queryMarkSent, id, now)
	return err
}

func (r Repository) MarkSentForOrganization(ctx context.Context, id, organizationID string) error {
	now := time.Now()
	tag, err := r.db.Exec(ctx, queryMarkSentForOrganization, id, now, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("notification not found")
	}
	return nil
}

func (r Repository) MarkFailed(ctx context.Context, id, errMsg string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx, queryMarkFailed, id, errMsg, now)
	return err
}

func (r Repository) IncrementRetryCount(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx, queryIncrementRetryCount, id, now)
	return err
}

func (r Repository) ListPendingRetries(ctx context.Context, limit int) ([]*domain.Notification, error) {
	rows, err := r.db.Query(ctx, queryListPendingRetries, limit)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list pending retries")
	}
	defer rows.Close()

	items := make([]*domain.Notification, 0)
	for rows.Next() {
		var row notificationRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.ProviderID,
			&row.UserNotificationID, &row.Channel, &row.ProviderKind,
			&row.Recipient,
			&row.Subject, &row.Content, &row.TemplateID, &row.Variables,
			&row.Status, &row.Error, &row.SentAt, &row.RetryCount,
			&row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan notification row")
		}
		n := toNotification(row)
		items = append(items, &n)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, apperrors.WrapInternalf(rowsErr, "iterate pending retries")
	}

	return items, nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

var _ domain.Repository = Repository{}

// maxInt returns value when positive, otherwise fallback. Kept package-local
// (and renamed from max) so it does not shadow the predeclared max builtin.
func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
