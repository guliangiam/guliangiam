package notifications

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testID     = "00000000-0000-0000-0000-000000000010"
	testOrgID  = "00000000-0000-0000-0000-000000000001"
	testProvID = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// notificationCols: 17 列
func notificationCols() []string {
	return []string{
		"id", "organization_id", "provider_id", "user_notification_id", "channel", "provider_kind",
		"recipient", "subject", "content",
		"template_id", "variables", "status", "error", "sent_at", "retry_count",
		"created_at", "updated_at",
	}
}

func newNotificationRow() []driver.Value {
	return []driver.Value{
		testID, testOrgID, testProvID, "", "", "",
		"user@example.com", "Subject", "Content",
		"tmpl-1", `{"key":"val"}`, "pending", nil, nil, 0,
		fixedNow, fixedNow,
	}
}

// ── GetByID ─────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetNotificationByID)).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows(notificationCols()).AddRow(newNotificationRow()...))

	r := NewRepository(db)
	n, err := r.GetByID(context.Background(), testID)
	require.NoError(t, err)
	require.NotNil(t, n)
	assert.Equal(t, testID, n.ID)
	assert.Equal(t, "Subject", n.Subject)
	assert.Equal(t, "pending", n.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetNotificationByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(notificationCols()))

	r := NewRepository(db)
	_, err := r.GetByID(context.Background(), "bad")
	require.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── List ─────────────────────────────────────────────────────────

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	orgID := testOrgID
	status := ""
	keyword := ""
	pageSize := 20
	offset := 0

	mock.ExpectQuery(regexp.QuoteMeta(queryListNotifications)).
		WithArgs(
			orgID, orgID,
			status, status,
			keyword, keyword, keyword,
			pageSize, offset,
		).
		WillReturnRows(sqlmock.NewRows(notificationCols()).AddRow(newNotificationRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountNotifications)).
		WithArgs(
			orgID, orgID,
			status, status,
			keyword, keyword, keyword,
		).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), orgID, status, keyword, 1, pageSize)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertNotification)).
		WithArgs(
			testID, testOrgID, testProvID, "", "", "", "user@example.com",
			"Subject", "Content", "tmpl-1", `{"key":"val"}`,
			"pending", nil, nil, 0,
			fixedNow, fixedNow,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Notification{
		ID:             testID,
		OrganizationID: testOrgID,
		ProviderID:     testProvID,
		Recipient:      "user@example.com",
		Subject:        "Subject",
		Content:        "Content",
		TemplateID:     "tmpl-1",
		Variables:      map[string]any{"key": "val"},
		Status:         "pending",
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ───────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateNotification)).
		WithArgs(
			testProvID, "user@example.com",
			"Subject", "Content", "tmpl-1",
			`{"key":"val"}`, "pending", nil,
			nil, 0, fixedNow,
			testID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Notification{
		ID:         testID,
		ProviderID: testProvID,
		Recipient:  "user@example.com",
		Subject:    "Subject",
		Content:    "Content",
		TemplateID: "tmpl-1",
		Variables:  map[string]any{"key": "val"},
		Status:     "pending",
		UpdatedAt:  fixedNow,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteNotification)).
		WithArgs(testID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testID)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── MarkSent ──────────────────────────────────────────────────────

func TestRepository_MarkSent(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryMarkSent)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), testID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.MarkSent(context.Background(), testID)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── MarkFailed ─────────────────────────────────────────────────────

func TestRepository_MarkFailed(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryMarkFailed)).
		WithArgs("error occurred", sqlmock.AnyArg(), testID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.MarkFailed(context.Background(), testID, "error occurred")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── IncrementRetryCount ────────────────────────────────────────────

func TestRepository_IncrementRetryCount(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryIncrementRetryCount)).
		WithArgs(sqlmock.AnyArg(), testID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.IncrementRetryCount(context.Background(), testID)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── ListPendingRetries ─────────────────────────────────────────────

func TestRepository_ListPendingRetries(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListPendingRetries)).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows(notificationCols()).AddRow(newNotificationRow()...))

	r := NewRepository(db)
	items, err := r.ListPendingRetries(context.Background(), 5)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
