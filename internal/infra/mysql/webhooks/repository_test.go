package webhooks

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow        = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testWebhookID   = "00000000-0000-0000-0000-000000000001"
	testOrgID       = "00000000-0000-0000-0000-000000000002"
	testWebhookName = "github-pr-sync"
	testDisplayName = "GitHub PR Sync"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义 ────────────────────────────────────────────────────────
// 共 29 列，与 scanWebhookRow 的 Scan 参数顺序严格一致。

func webhookCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name",
		"url", "method", "headers", "events",
		"is_enabled", "secret", "retry_count", "timeout_ms",
		"organization", "content_type", "custom_body", "custom_headers",
		"cert_id", "is_error", "error_text",
		"token_fields", "object_fields", "is_user_extended",
		"single_org_only", "max_retries", "retry_interval", "use_exponential_backoff",
		"created_at", "updated_at", "revision",
	}
}

// newWebhookRow 严格匹配 scanWebhookRow 的 29 个 Scan 参数顺序。
// secret 在 DB 层存密文（加密开启时），用 webhookEncryptedMapper 动态加密。
func newWebhookRow() []driver.Value {
	encSecret, _ := webhookEncryptedMapper.EncryptColumn(webhookAAD(testWebhookID), "s3cret")
	return []driver.Value{
		testWebhookID, testOrgID, testWebhookName, testDisplayName,
		"https://api.example.com/webhook", "POST", `{"X-Custom":"val"}`, []byte(`["pull_request","push"]`),
		true, encSecret, 0, 5000,
		"", "application/json", "", "{}",
		"", false, "",
		[]byte(`[]`), []byte(`[]`), false,
		false, 3, 60, false,
		fixedNow, fixedNow, int64(1),
	}
}

func Test_colCounts_Webhooks(t *testing.T) {
	t.Logf("webhookCols count: %d", len(webhookCols()))
	t.Logf("webhookRow count:  %d", len(newWebhookRow()))
	assert.Equal(t, len(webhookCols()), len(newWebhookRow()), "cols vs row count mismatch")
}

// ── GetByID ──────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetWebhookByID)).
		WithArgs(testWebhookID).
		WillReturnRows(sqlmock.NewRows(webhookCols()).AddRow(newWebhookRow()...))

	r := NewRepository(db)
	w, err := r.GetByID(context.Background(), testWebhookID)
	assert.NoError(t, err)
	assert.NotNil(t, w)
	assert.Equal(t, testWebhookID, w.ID)
	assert.Equal(t, testOrgID, w.OrganizationID)
	assert.Equal(t, testWebhookName, w.Name)
	assert.Equal(t, testDisplayName, w.DisplayName)
	assert.Equal(t, "https://api.example.com/webhook", w.URL)
	assert.Equal(t, "POST", w.Method)
	assert.True(t, w.IsEnabled)
	assert.Equal(t, []string{"pull_request", "push"}, w.Events)
	assert.Equal(t, int64(1), w.Revision)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetWebhookByID)).
		WithArgs("unknown-id").
		WillReturnRows(sqlmock.NewRows(webhookCols()))

	r := NewRepository(db)
	w, err := r.GetByID(context.Background(), "unknown-id")
	assert.Error(t, err)
	assert.Nil(t, w)
}

// ── List ─────────────────────────────────────────────────────────
// queryListWebhooks 参数：orgID×2, keyword×4, is_enabled×2, method×2, limit, offset = 12
// queryCountWebhooks 参数：orgID×2, keyword×4, is_enabled×2, method×2 = 10

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 默认 ListQuery：空 orgID、空 keyword、nil is_enabled、空 method、Page=1、PageSize=20
	// orgID × 2 = "", ""; keyword × 4 = "", "", "", ""; is_enabled × 2 = nil, nil; method × 2 = "", ""
	// limit=20, offset=0
	mock.ExpectQuery(regexp.QuoteMeta(queryListWebhooks)).
		WithArgs("", "", "", "", "", "", nil, nil, "", "", 20, 0).
		WillReturnRows(sqlmock.NewRows(webhookCols()).AddRow(newWebhookRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountWebhooks)).
		WithArgs("", "", "", "", "", "", nil, nil, "", "").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, testWebhookID, items[0].ID)
}

func TestRepository_List_WithFilter(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	isEnabled := true

	mock.ExpectQuery(regexp.QuoteMeta(queryListWebhooks)).
		WithArgs(testOrgID, testOrgID, "github", "github", "github", "github", isEnabled, isEnabled, "POST", "POST", 20, 0).
		WillReturnRows(sqlmock.NewRows(webhookCols()))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountWebhooks)).
		WithArgs(testOrgID, testOrgID, "github", "github", "github", "github", isEnabled, isEnabled, "POST", "POST").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		Page:           1,
		PageSize:       20,
		OrganizationID: testOrgID,
		Keyword:        "github",
		IsEnabled:      &isEnabled,
		Method:         "POST",
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 29 个 ? 参数
	mock.ExpectExec(regexp.QuoteMeta(queryInsertWebhook)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	w := &domain.Webhook{
		ID:             testWebhookID,
		OrganizationID: testOrgID,
		Name:           testWebhookName,
		DisplayName:    testDisplayName,
		URL:            "https://api.example.com/webhook",
		Method:         "POST",
		IsEnabled:      true,
		Events:         []string{"pull_request", "push"},
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       1,
	}
	err := r.Create(context.Background(), w)
	assert.NoError(t, err)
}

// ── Update ───────────────────────────────────────────────────────
// queryUpdateWebhook: 26 个 ?（display_name..updated_at = 24 + id + revision = 2）
// querySelectRevision: 1 个 ?（id）

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateWebhook)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testWebhookID, int64(1), // id + revision
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevision)).
		WithArgs(testWebhookID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	w := &domain.Webhook{
		ID:          testWebhookID,
		DisplayName: "Updated Display",
		Revision:    1,
		UpdatedAt:   fixedNow,
	}
	err := r.Update(context.Background(), w)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), w.Revision)
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateWebhook)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testWebhookID, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 rows affected = conflict

	r := NewRepository(db)
	w := &domain.Webhook{
		ID:          testWebhookID,
		DisplayName: "Updated",
		Revision:    1,
		UpdatedAt:   fixedNow,
	}
	err := r.Update(context.Background(), w)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

// ── Delete ───────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteWebhook)).
		WithArgs(testWebhookID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testWebhookID)
	assert.NoError(t, err)
}

// ── ListByOrganizationAndEvent ──────────────────────────────────

func TestRepository_ListByOrganizationAndEvent(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	eventJSON := []byte(`["pull_request"]`)

	mock.ExpectQuery(regexp.QuoteMeta(queryListWebhooksByOrgAndEvent)).
		WithArgs(testOrgID, eventJSON).
		WillReturnRows(sqlmock.NewRows(webhookCols()).AddRow(newWebhookRow()...))

	r := NewRepository(db)
	items, err := r.ListByOrganizationAndEvent(context.Background(), testOrgID, "pull_request")
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, testWebhookID, items[0].ID)
	assert.Equal(t, []string{"pull_request", "push"}, items[0].Events)
}

func TestRepository_ListByOrganizationAndEvent_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	eventJSON := []byte(`["unknown"]`)

	mock.ExpectQuery(regexp.QuoteMeta(queryListWebhooksByOrgAndEvent)).
		WithArgs(testOrgID, eventJSON).
		WillReturnRows(sqlmock.NewRows(webhookCols()))

	r := NewRepository(db)
	items, err := r.ListByOrganizationAndEvent(context.Background(), testOrgID, "unknown")
	assert.NoError(t, err)
	assert.Empty(t, items)
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}

// TestRepository_GetByIDForOrganization_TenantPredicate pins the explicit
// organization_id predicate on the MySQL by-ID read.
func TestRepository_GetByIDForOrganization_TenantPredicate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetWebhookByIDForOrganization)).
		WithArgs(testWebhookID, testOrgID).
		WillReturnRows(sqlmock.NewRows(webhookCols()))

	_, err := NewRepository(db).GetByIDForOrganization(context.Background(), testWebhookID, testOrgID)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
