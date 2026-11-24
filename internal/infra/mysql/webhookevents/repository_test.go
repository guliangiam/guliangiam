package webhookevents

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow       = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedRetryTime = fixedNow.Add(time.Minute)
	testEventID    = "00000000-0000-0000-0000-000000000001"
	testWebhookID  = "00000000-0000-0000-0000-000000000002"
	testEventType  = "pull_request"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义 ────────────────────────────────────────────────────────
// 共 13 列，与 scanWebhookEventRow 的 Scan 参数顺序严格一致。

func eventCols() []string {
	return []string{
		"id", "webhook_id", "event_type", "payload",
		"response_code", "response_body", "status", "retry_count",
		"next_retry_at", "idempotency_key",
		"created_at", "updated_at", "revision",
	}
}

// newEventRow 严格匹配 scanWebhookEventRow 的 13 个 Scan 参数顺序。
func newEventRow() []driver.Value {
	return []driver.Value{
		testEventID, testWebhookID, testEventType, []byte(`{"pr":1}`),
		sql.NullInt64{Int64: 200, Valid: true}, sql.NullString{String: "OK", Valid: true},
		"succeeded", 0,
		sql.NullTime{}, sql.NullString{},
		fixedNow, fixedNow, int64(1),
	}
}

func Test_colCounts_WebhookEvents(t *testing.T) {
	t.Logf("eventCols count: %d", len(eventCols()))
	t.Logf("eventRow count:  %d", len(newEventRow()))
	assert.Equal(t, len(eventCols()), len(newEventRow()), "cols vs row count mismatch")
}

// ── GetByID ──────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs(testEventID).
		WillReturnRows(sqlmock.NewRows(eventCols()).AddRow(newEventRow()...))

	r := NewRepository(db)
	e, err := r.GetByID(context.Background(), testEventID)
	assert.NoError(t, err)
	assert.NotNil(t, e)
	assert.Equal(t, testEventID, e.ID)
	assert.Equal(t, testWebhookID, e.WebhookID)
	assert.Equal(t, testEventType, e.EventType)
	assert.Equal(t, 200, e.ResponseCode)
	assert.Equal(t, "OK", e.ResponseBody)
	assert.Equal(t, "succeeded", e.Status)
	assert.Equal(t, int64(1), e.Revision)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs("unknown-id").
		WillReturnRows(sqlmock.NewRows(eventCols()))

	r := NewRepository(db)
	e, err := r.GetByID(context.Background(), "unknown-id")
	assert.Error(t, err)
	assert.Nil(t, e)
}

func TestRepository_GetByID_AllNullable(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 所有 NULL 列都是零值 Null*
	row := []driver.Value{
		testEventID, testWebhookID, testEventType, []byte(`{"pr":2}`),
		sql.NullInt64{}, sql.NullString{}, // response_code, response_body NULL
		"pending", 3,
		sql.NullTime{}, sql.NullString{}, // next_retry_at, idempotency_key NULL
		fixedNow, fixedNow, int64(1),
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs(testEventID).
		WillReturnRows(sqlmock.NewRows(eventCols()).AddRow(row...))

	r := NewRepository(db)
	e, err := r.GetByID(context.Background(), testEventID)
	assert.NoError(t, err)
	assert.Equal(t, 0, e.ResponseCode)  // Null → 0
	assert.Equal(t, "", e.ResponseBody) // Null → ""
	assert.Nil(t, e.NextRetryAt)        // Null → nil
	assert.Equal(t, "", e.IdempotencyKey)
}

// ── List ─────────────────────────────────────────────────────────
// queryList: webhook_id×2, event_type×2, status×2, organization_id×2, limit, offset = 10
// queryCount: webhook_id×2, event_type×2, status×2, organization_id×2 = 8

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryList)).
		WithArgs("", "", "", "", nil, nil, "", "", 20, 0).
		WillReturnRows(sqlmock.NewRows(eventCols()).AddRow(newEventRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCount)).
		WithArgs("", "", "", "", nil, nil, "", "").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, testEventID, items[0].ID)
}

func TestRepository_List_WithFilter(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	status := "pending"

	mock.ExpectQuery(regexp.QuoteMeta(queryList)).
		WithArgs(testWebhookID, testWebhookID, testEventType, testEventType, status, status, "", "", 20, 0).
		WillReturnRows(sqlmock.NewRows(eventCols()))

	mock.ExpectQuery(regexp.QuoteMeta(queryCount)).
		WithArgs(testWebhookID, testWebhookID, testEventType, testEventType, status, status, "", "").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		Page:      1,
		PageSize:  20,
		WebhookID: testWebhookID,
		EventType: testEventType,
		Status:    &status,
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

// MySQL has no RLS and webhook_events has no organization_id column, so the
// repo must bind the tenant explicitly through the parent webhook. A cross-
// tenant parent yields no rows and the count agrees with the empty item list.
func TestRepository_List_TenantScopedToParentOwner(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	const tenantOrg = "11111111-1111-1111-1111-111111111111"
	const foreignWebhook = "33333333-3333-3333-3333-333333333333"

	mock.ExpectQuery(regexp.QuoteMeta(queryList)).
		WithArgs(foreignWebhook, foreignWebhook, "", "", nil, nil, tenantOrg, tenantOrg, 20, 0).
		WillReturnRows(sqlmock.NewRows(eventCols()))

	mock.ExpectQuery(regexp.QuoteMeta(queryCount)).
		WithArgs(foreignWebhook, foreignWebhook, "", "", nil, nil, tenantOrg, tenantOrg).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		Page:           1,
		PageSize:       20,
		WebhookID:      foreignWebhook,
		OrganizationID: tenantOrg,
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// An empty tenant is the platform all-tenant scope (only reachable after the
// app-layer scope check); it must not become a rogue CHAR(36) comparison.
func TestRepository_List_EmptyTenantKeepsAllTenantScope(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryList)).
		WithArgs("", "", "", "", nil, nil, "", "", 20, 0).
		WillReturnRows(sqlmock.NewRows(eventCols()))
	mock.ExpectQuery(regexp.QuoteMeta(queryCount)).
		WithArgs("", "", "", "", nil, nil, "", "").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	_, _, err := NewRepository(db).List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
}

func Test_QueryList_And_QueryCount_ShareTenantPredicate(t *testing.T) {
	snippet := "(? = '' OR EXISTS (SELECT 1 FROM " + btWebhooks +
		" w WHERE w.id = webhook_events.webhook_id AND w.organization_id = ?))"
	assert.Contains(t, queryList, snippet)
	assert.Contains(t, queryCount, snippet)
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsert)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	nextRetry := fixedRetryTime
	e := &domain.WebhookEvent{
		ID:          testEventID,
		WebhookID:   testWebhookID,
		EventType:   testEventType,
		Payload:     nil, // 走默认 {}
		Status:      "pending",
		RetryCount:  3,
		NextRetryAt: &nextRetry,
		CreatedAt:   fixedNow,
		UpdatedAt:   fixedNow,
		Revision:    1,
	}
	err := r.Create(context.Background(), e)
	assert.NoError(t, err)
}

// ── ListExistingIDs ──────────────────────────────────────────────

func TestRepository_ListExistingIDs_EmptyInputShortCircuits(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	found, err := NewRepository(db).ListExistingIDs(context.Background(), nil)
	assert.NoError(t, err)
	assert.Nil(t, found)
	assert.NoError(t, mock.ExpectationsWereMet(), "空入参不得触达数据库")
}

func TestRepository_ListExistingIDs(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	const otherID = "00000000-0000-0000-0000-000000000009"
	query := queryListExistingIDsPrefix + "?,?" + ")"
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(testEventID, otherID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testEventID))

	found, err := NewRepository(db).ListExistingIDs(context.Background(), []string{testEventID, otherID})
	assert.NoError(t, err)
	assert.Equal(t, []string{testEventID}, found)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// MySQL CHAR(36) 匹配依赖 collation，绑定 canonical 小写形式保证与 PG 行为
// 等价（domain Repository 契约）。
func TestRepository_ListExistingIDs_CanonicalizesInput(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	const mixed = "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"
	const canonical = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	query := queryListExistingIDsPrefix + "?,?" + ")"
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(canonical, testEventID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(canonical))

	found, err := NewRepository(db).ListExistingIDs(context.Background(), []string{" " + mixed + " ", "  ", testEventID})
	assert.NoError(t, err)
	assert.Equal(t, []string{canonical}, found)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ListExistingIDs_BlankOnlyShortCircuits(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	found, err := NewRepository(db).ListExistingIDs(context.Background(), []string{"", "  "})
	assert.NoError(t, err)
	assert.Nil(t, found)
	assert.NoError(t, mock.ExpectationsWereMet(), "全空入参不得触达数据库")
}

// IN 列表按 maxIDsPerQuery 分块，避免超过 prepared-statement 占位符上限。
func TestRepository_ListExistingIDs_ChunksINList(t *testing.T) {
	old := maxIDsPerQuery
	maxIDsPerQuery = 2
	t.Cleanup(func() { maxIDsPerQuery = old })

	db, mock := newMockDB(t)
	defer db.Close()

	ids := []string{testEventID, testWebhookID, "00000000-0000-0000-0000-000000000009"}
	mock.ExpectQuery(regexp.QuoteMeta(queryListExistingIDsPrefix+"?,?"+")")).
		WithArgs(ids[0], ids[1]).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(ids[0]))
	mock.ExpectQuery(regexp.QuoteMeta(queryListExistingIDsPrefix + "?)")).
		WithArgs(ids[2]).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	found, err := NewRepository(db).ListExistingIDs(context.Background(), ids)
	assert.NoError(t, err)
	assert.Equal(t, []string{ids[0]}, found)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── CreateBatch ──────────────────────────────────────────────────

func TestRepository_CreateBatch_EmptyShortCircuits(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	err := NewRepository(db).CreateBatch(context.Background(), nil)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet(), "空批次不得触达数据库")
}

func TestRepository_CreateBatch_TwoRows(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	row := "(?,?,?,?,?,?,?,?,?,?,?,?,?)"
	query := queryInsertBatchPrefix + row + "," + row
	// 第一行精确钉 13 列同序与 NULL 语义，第二行 AnyArg 钉数量与形状。
	args := make([]driver.Value, 0, 26)
	args = append(args,
		testEventID, testWebhookID, testEventType, []byte("{}"),
		nil, nil, "pending", int64(0),
		nil, nil, fixedNow, fixedNow, int64(1),
	)
	for i := 0; i < 13; i++ {
		args = append(args, sqlmock.AnyArg())
	}
	mock.ExpectExec(regexp.QuoteMeta(query)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(2, 2))

	r := NewRepository(db)
	err := r.CreateBatch(context.Background(), []*domain.WebhookEvent{
		{ID: testEventID, WebhookID: testWebhookID, EventType: testEventType, Status: "pending", CreatedAt: fixedNow, UpdatedAt: fixedNow, Revision: 1},
		{ID: "00000000-0000-0000-0000-000000000003", WebhookID: testWebhookID, EventType: testEventType, Status: "pending", ResponseCode: 200, CreatedAt: fixedNow, UpdatedAt: fixedNow, Revision: 1},
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 多行 INSERT 按 maxBatchInsertRows 分块（13 列 × 块大小 < 65,535 占位符上限），
// 整体原子性由调用方事务保证。
func TestRepository_CreateBatch_ChunksRows(t *testing.T) {
	old := maxBatchInsertRows
	maxBatchInsertRows = 2
	t.Cleanup(func() { maxBatchInsertRows = old })

	db, mock := newMockDB(t)
	defer db.Close()

	row := "(?,?,?,?,?,?,?,?,?,?,?,?,?)"
	query := queryInsertBatchPrefix + row + "," + row
	mock.ExpectExec(regexp.QuoteMeta(query)).
		WithArgs(makeMockArgs(26)...).
		WillReturnResult(sqlmock.NewResult(2, 2))
	mock.ExpectExec(regexp.QuoteMeta(queryInsertBatchPrefix + row)).
		WithArgs(makeMockArgs(13)...).
		WillReturnResult(sqlmock.NewResult(1, 1))

	events := make([]*domain.WebhookEvent, 3)
	for i := range events {
		events[i] = &domain.WebhookEvent{ID: fmt.Sprintf("00000000-0000-0000-0000-%012d", i)}
	}
	err := NewRepository(db).CreateBatch(context.Background(), events)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func makeMockArgs(n int) []driver.Value {
	args := make([]driver.Value, 0, n)
	for i := 0; i < n; i++ {
		args = append(args, sqlmock.AnyArg())
	}
	return args
}

// ── Update ───────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdate)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testEventID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	e := &domain.WebhookEvent{
		ID:           testEventID,
		Status:       "succeeded",
		ResponseCode: 200,
		ResponseBody: "OK",
		UpdatedAt:    fixedNow,
		Revision:     2,
	}
	err := r.Update(context.Background(), e)
	assert.NoError(t, err)
}

// ── Delete ───────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDelete)).
		WithArgs(testEventID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testEventID)
	assert.NoError(t, err)
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}

// TestRepository_GetByIDForOrganization_ParentTenantPredicate pins that the
// MySQL event read binds organization_id through the parent webhook EXISTS.
func TestRepository_GetByIDForOrganization_ParentTenantPredicate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByIDForOrganization)).
		WithArgs(testEventID, "org-1").
		WillReturnRows(sqlmock.NewRows(eventCols()))

	_, err := NewRepository(db).GetByIDForOrganization(context.Background(), testEventID, "org-1")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
