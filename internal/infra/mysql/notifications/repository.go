package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 notifications 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// notificationRow 是 notifications 表扫描目标。
// Variables PG jsonb → MySQL TEXT (JSON 字符串)
type notificationRow struct {
	ID                 string
	OrganizationID     string
	ProviderID         string
	UserNotificationID string
	Channel            string
	ProviderKind       string
	Recipient          string
	Subject            sql.NullString
	Content            string
	TemplateID         sql.NullString
	Variables          string // JSON → map[string]any
	Status             string
	Error              sql.NullString
	SentAt             *time.Time
	RetryCount         int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// nullableString 把空字符串转 nil。
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// scanNotification 扫描一行到 domain.Notification。
func scanNotification(scanner interface{ Scan(...any) error }) (*domain.Notification, error) {
	var row notificationRow
	err := scanner.Scan(
		&row.ID, &row.OrganizationID, &row.ProviderID,
		&row.UserNotificationID, &row.Channel, &row.ProviderKind,
		&row.Recipient,
		&row.Subject, &row.Content, &row.TemplateID, &row.Variables,
		&row.Status, &row.Error, &row.SentAt, &row.RetryCount,
		&row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scan notification")
	}
	return toNotification(row), nil
}

// GetByID 按 id 查 Notification（系统级；请求路径必须使用 scoped 版本）。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	return scanNotification(r.db.QueryRowContext(ctx, queryGetNotificationByID, id))
}

// GetByIDForOrganization 按 id + 组织查 Notification；跨租户与不存在统一 NotFound。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Notification, error) {
	var row notificationRow
	err := r.db.QueryRowContext(ctx, queryGetNotificationByIDForOrganization, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &row.ProviderID,
		&row.UserNotificationID, &row.Channel, &row.ProviderKind,
		&row.Recipient,
		&row.Subject, &row.Content, &row.TemplateID, &row.Variables,
		&row.Status, &row.Error, &row.SentAt, &row.RetryCount,
		&row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("notification not found")
		}
		return nil, apperrors.WrapInternalf(err, "get notification by id for organization")
	}
	return toNotification(row), nil
}

// List 分页查询 Notifications。
// 占位符：org_id×2(2), status×2(2), keyword×3(3), limit, offset(2) = 9
func (r Repository) List(ctx context.Context, organizationID, status, keyword string, page, pageSize int) ([]*domain.Notification, int64, error) {
	offset := mysqlshared.PageOffset(page, pageSize)
	limit := mysqlshared.ClampPageSize(pageSize)

	listArgs := []any{
		organizationID, organizationID, // CASE WHEN org
		status, status, // CASE WHEN status
		keyword, keyword, keyword, // keyword (1 CASE + 2 LIKE)
		limit, offset,
	}

	rows, err := r.db.QueryContext(ctx, queryListNotifications, listArgs...)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "list notifications")
	}
	defer rows.Close()

	items := make([]*domain.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, n)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, apperrors.WrapInternalf(rowsErr, "iterate notifications")
	}

	countArgs := []any{
		organizationID, organizationID,
		status, status,
		keyword, keyword, keyword,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountNotifications, countArgs...).Scan(&total); err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "count notifications")
	}

	return items, total, nil
}

// Create 插入 Notification。
func (r Repository) Create(ctx context.Context, notification *domain.Notification) error {
	variablesJSON, err := json.Marshal(notification.Variables)
	if err != nil {
		return apperrors.NewInternal("encode variables")
	}

	_, err = r.db.ExecContext(ctx, queryInsertNotification,
		notification.ID, notification.OrganizationID, notification.ProviderID,
		notification.UserNotificationID, notification.Channel, notification.ProviderKind,
		notification.Recipient, nullableString(notification.Subject), notification.Content,
		nullableString(notification.TemplateID), string(variablesJSON), notification.Status,
		nullableString(notification.Error), notification.SentAt, notification.RetryCount,
		notification.CreatedAt, notification.UpdatedAt,
	)
	return err
}

// Update 更新 Notification。
func (r Repository) Update(ctx context.Context, notification *domain.Notification) error {
	variablesJSON, err := json.Marshal(notification.Variables)
	if err != nil {
		return apperrors.NewInternal("encode variables")
	}

	_, err = r.db.ExecContext(ctx, queryUpdateNotification,
		notification.ProviderID, notification.Recipient,
		notification.Subject, notification.Content, nullableString(notification.TemplateID),
		string(variablesJSON), notification.Status, nullableString(notification.Error),
		notification.SentAt, notification.RetryCount, notification.UpdatedAt,
		notification.ID,
	)
	return err
}

// Delete 按 id 删除 Notification。
func (r Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteNotification, id)
	return err
}

// MarkSent 标记 notification 为已发送。
func (r Repository) MarkSent(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, queryMarkSent, now, now, id)
	return err
}

// MarkSentForOrganization 仅在同租户内标记 notification 为已发送；0 行统一 NotFound。
func (r Repository) MarkSentForOrganization(ctx context.Context, id, organizationID string) error {
	now := time.Now()
	result, err := r.db.ExecContext(ctx, queryMarkSentForOrganization, now, now, id, organizationID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("notification not found")
	}
	return nil
}

// MarkFailed 标记 notification 为失败。
func (r Repository) MarkFailed(ctx context.Context, id, errMsg string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, queryMarkFailed, errMsg, now, id)
	return err
}

// IncrementRetryCount 递增重试计数。
func (r Repository) IncrementRetryCount(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, queryIncrementRetryCount, now, id)
	return err
}

// ListPendingRetries 列出待重试的 notifications。
func (r Repository) ListPendingRetries(ctx context.Context, limit int) ([]*domain.Notification, error) {
	rows, err := r.db.QueryContext(ctx, queryListPendingRetries, limit)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list pending retries")
	}
	defer rows.Close()

	items := make([]*domain.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, apperrors.WrapInternalf(rowsErr, "iterate pending retries")
	}
	return items, nil
}

// toNotification 把 MySQL row 转成 domain.Notification。
func toNotification(row notificationRow) *domain.Notification {
	n := &domain.Notification{
		ID:                 row.ID,
		OrganizationID:     row.OrganizationID,
		ProviderID:         row.ProviderID,
		UserNotificationID: row.UserNotificationID,
		Channel:            row.Channel,
		ProviderKind:       row.ProviderKind,
		Recipient:          row.Recipient,
		Content:            row.Content,
		Status:             row.Status,
		SentAt:             row.SentAt,
		RetryCount:         row.RetryCount,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
	if row.Subject.Valid {
		n.Subject = row.Subject.String
	}
	if row.TemplateID.Valid {
		n.TemplateID = row.TemplateID.String
	}
	if row.Error.Valid {
		n.Error = row.Error.String
	}
	if row.Variables != "" {
		_ = json.Unmarshal([]byte(row.Variables), &n.Variables)
	}
	return n
}

// 编译期接口断言
var _ domain.Repository = Repository{}
