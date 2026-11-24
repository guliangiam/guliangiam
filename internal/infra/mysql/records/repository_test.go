package records

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow    = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testRecID   = "00000000-0000-0000-0000-000000000001"
	testOrgID   = "00000000-0000-0000-0000-000000000002"
	testActor   = "00000000-0000-0000-0000-000000000003"
	testAction  = "login"
	testResType = "user"
	testResID   = "00000000-0000-0000-0000-000000000004"
)

// emptyKw 返回 6 个空字符串（recordsKeyword 包含 6 个 ?）
func emptyKw() []driver.Value {
	return []driver.Value{"", "", "", "", "", ""}
}

// kw 返回 6 个相同 keyword 值
func kw(k string) []driver.Value {
	return []driver.Value{k, k, k, k, k, k}
}

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义（与 recordRow 扫描顺序严格一致，共 13 列）────────

func recCols() []string {
	return []string{
		"id", "organization_id", "actor_id", "action", "resource_type", "resource_id",
		"ip", "user_agent", "surface", "request_id", "trace_id", "metadata", "created_at",
	}
}

// newRecRow 严格匹配 scanRecordRow 的 13 个 Scan 参数顺序。
func newRecRow() []driver.Value {
	return []driver.Value{
		testRecID, testOrgID, testActor, testAction, testResType, testResID,
		"127.0.0.1", "test-ua", "management", "req-001", "trace-001",
		[]byte(`{"key":"value"}`), fixedNow,
	}
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 17 个 ? 参数
	mock.ExpectExec(regexp.QuoteMeta(queryCreateRecord)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			"", int64(0), nil, nil, // chain fields
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	rec := &domain.Record{
		ID:             testRecID,
		OrganizationID: testOrgID,
		ActorID:        testActor,
		Action:         testAction,
		ResourceType:   testResType,
		ResourceID:     testResID,
		IP:             "127.0.0.1",
		UserAgent:      "test-ua",
		Surface:        "management",
		RequestID:      "req-001",
		TraceID:        "trace-001",
		Metadata:       map[string]any{"key": "value"},
		CreatedAt:      fixedNow,
	}
	err := r.Create(context.Background(), rec)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── List ────────────────────────────────────────────────────────
// List 参数（21 个）：org_id×2(2), actor_id×2(2), action×2(2), resource_type×2(2), keyword×11, limit, offset(2)
// Count 参数（19 个）：同上，去掉 limit/offset

func TestRepository_List_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	listArgs := make([]driver.Value, 0, 21)
	listArgs = append(listArgs,
		"", "", "", "", "", "", "", "", // 8
	)
	listArgs = append(listArgs, emptyKw()...) // 11
	listArgs = append(listArgs, 20, 0)        // 2 → 21

	mock.ExpectQuery(regexp.QuoteMeta(queryListRecords)).
		WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(recCols()).AddRow(newRecRow()...))

	countArgs := make([]driver.Value, 0, 19)
	countArgs = append(countArgs,
		"", "", "", "", "", "", "", "", // 8
	)
	countArgs = append(countArgs, emptyKw()...) // 11 → 19

	mock.ExpectQuery(regexp.QuoteMeta(queryCountRecords)).
		WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, testRecID, items[0].ID)
	assert.Equal(t, "login", items[0].Action)
	assert.NotNil(t, items[0].Metadata)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	listArgs := make([]driver.Value, 0, 21)
	listArgs = append(listArgs, "", "", "", "", "", "", "", "")
	listArgs = append(listArgs, emptyKw()...)
	listArgs = append(listArgs, 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListRecords)).WithArgs(listArgs...).WillReturnRows(sqlmock.NewRows(recCols()))

	countArgs := make([]driver.Value, 0, 19)
	countArgs = append(countArgs, "", "", "", "", "", "", "", "")
	countArgs = append(countArgs, emptyKw()...)
	mock.ExpectQuery(regexp.QuoteMeta(queryCountRecords)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestRepository_List_WithFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	listArgs := make([]driver.Value, 0, 21)
	listArgs = append(listArgs,
		testOrgID, testOrgID,
		testActor, testActor,
		testAction, testAction,
		"user", "user",
	)
	listArgs = append(listArgs, kw("test")...)
	listArgs = append(listArgs, 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListRecords)).WithArgs(listArgs...).WillReturnRows(sqlmock.NewRows(recCols()))

	countArgs := make([]driver.Value, 0, 19)
	countArgs = append(countArgs,
		testOrgID, testOrgID,
		testActor, testActor,
		testAction, testAction,
		"user", "user",
	)
	countArgs = append(countArgs, kw("test")...)
	mock.ExpectQuery(regexp.QuoteMeta(queryCountRecords)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		Page:           1,
		PageSize:       20,
		OrganizationID: testOrgID,
		ActorID:        testActor,
		Action:         testAction,
		ResourceType:   "user",
		Keyword:        "test",
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Search ───────────────────────────────────────────────────────
// Search 参数（27 个）：
//   org_id×2(2), actor_id×2(2), action×2(2), resource_type×2(2), resource_id×2(2),
//   start_time×2(2), end_time×2(2), keyword×11, limit, offset(2)
//   总计: 8+2+4+11+2 = 27
// Count 参数（25 个）：同上，去掉 limit/offset

func TestRepository_Search_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	listArgs := make([]driver.Value, 0, 27)
	listArgs = append(listArgs,
		testOrgID, testOrgID,
		testActor, testActor,
		testAction, testAction,
		testResType, testResType,
		testResID, testResID,
		nil, nil,
		nil, nil,
	)
	listArgs = append(listArgs, emptyKw()...)
	listArgs = append(listArgs, 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(querySearchRecords)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(recCols()).AddRow(newRecRow()...))

	countArgs := make([]driver.Value, 0, 25)
	countArgs = append(countArgs,
		testOrgID, testOrgID,
		testActor, testActor,
		testAction, testAction,
		testResType, testResType,
		testResID, testResID,
		nil, nil, nil, nil,
	)
	countArgs = append(countArgs, emptyKw()...)
	mock.ExpectQuery(regexp.QuoteMeta(queryCountSearchRecords)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.Search(context.Background(), domain.SearchCriteria{
		Page:           1,
		PageSize:       20,
		OrganizationID: testOrgID,
		ActorID:        testActor,
		Action:         testAction,
		ResourceType:   testResType,
		ResourceID:     testResID,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Search_WithTimeRange(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	startTime := fixedNow.Add(-24 * time.Hour)
	endTime := fixedNow

	listArgs := make([]driver.Value, 0, 27)
	listArgs = append(listArgs,
		"", "", "", "", "", "", "", "",
		"", "", // resource_id × 2
		startTime, startTime,
		endTime, endTime,
	)
	listArgs = append(listArgs, kw("kw")...)
	listArgs = append(listArgs, 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(querySearchRecords)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(recCols()))

	countArgs := make([]driver.Value, 0, 25)
	countArgs = append(countArgs,
		"", "", "", "", "", "", "", "",
		"", "",
		startTime, startTime, endTime, endTime,
	)
	countArgs = append(countArgs, kw("kw")...)
	mock.ExpectQuery(regexp.QuoteMeta(queryCountSearchRecords)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.Search(context.Background(), domain.SearchCriteria{
		Page:      1,
		PageSize:  20,
		Keyword:   "kw",
		StartTime: &startTime,
		EndTime:   &endTime,
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
