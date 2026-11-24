package metering

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedFrom = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	fixedTo   = time.Date(2026, 9, 30, 23, 59, 59, 0, time.UTC)
	testOrgID = "00000000-0000-0000-0000-000000000001"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── Events 列定义（10 列） ────────────────────────────────────

func eventCols() []string {
	return []string{
		"id", "organization_id", "event_type", "subject", "quantity", "unit",
		"idempotency_key", "occurred_at", "metadata", "created_at",
	}
}

func newEventRow() []driver.Value {
	return []driver.Value{
		"ev-001", testOrgID, "login", "user-123", int64(1), "count",
		"idem-login-001", fixedNow, json.RawMessage(`{"ip":"1.2.3.4"}`), fixedNow,
	}
}

func eventFromRow() *domain.MeteringEvent {
	return &domain.MeteringEvent{
		ID:             "ev-001",
		OrganizationID: testOrgID,
		EventType:      "login",
		Subject:        "user-123",
		Quantity:       1,
		Unit:           "count",
		IdempotencyKey: "idem-login-001",
		OccurredAt:     fixedNow,
		Metadata:       json.RawMessage(`{"ip":"1.2.3.4"}`),
	}
}

// ── Aggregates 列定义（9 列） ─────────────────────────────────

func aggCols() []string {
	return []string{
		"id", "organization_id", "period", "subject", "metric", "quantity", "unit", "calculated_at", "created_at",
	}
}

func newAggRow() []driver.Value {
	return []driver.Value{
		"agg-001", testOrgID, "2026-09", "user-123", "login", int64(42), "count", fixedNow, fixedNow,
	}
}

func aggFromRow() *domain.MeteringAggregate {
	return &domain.MeteringAggregate{
		ID:             "agg-001",
		OrganizationID: testOrgID,
		Period:         "2026-09",
		Subject:        "user-123",
		Metric:         "login",
		Quantity:       42,
		Unit:           "count",
	}
}

// ── ListEvents（默认，无 filter） ──────────────────────────────

func TestRepository_ListEvents_Default(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// countSQL: WHERE organization_id = ? → 1 个参数
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `metering_events` WHERE organization_id = ?")).
		WithArgs(testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	// selectSQL: WHERE organization_id = ? ORDER BY ... LIMIT ? OFFSET ? → 3 个参数
	mock.ExpectQuery(regexp.QuoteMeta(" FROM `metering_events` WHERE organization_id = ? ORDER BY occurred_at DESC LIMIT ? OFFSET ?")).
		WithArgs(testOrgID, 20, 0).
		WillReturnRows(sqlmock.NewRows(eventCols()).AddRow(newEventRow()...))

	r := NewRepository(db)
	items, total, err := r.ListEvents(context.Background(), domain.ListEventsQuery{
		OrganizationID: testOrgID,
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "ev-001", items[0].ID)
}

// ── ListEvents（全 filter） ────────────────────────────────────

func TestRepository_ListEvents_WithFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	eventType := "login"
	subject := "user-123"

	// countSQL: org + event_type + subject + from + to = 5 个 ?
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `metering_events` WHERE organization_id = ?")).
		WithArgs(testOrgID, eventType, subject, fixedFrom, fixedTo).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(2)))

	// selectSQL: 同上 + limit + offset = 7 个 ?
	mock.ExpectQuery(regexp.QuoteMeta(`WHERE organization_id = ?`)).
		WithArgs(testOrgID, eventType, subject, fixedFrom, fixedTo, 10, 0).
		WillReturnRows(sqlmock.NewRows(eventCols()).AddRow(newEventRow()...).AddRow(newEventRow()...))

	r := NewRepository(db)
	items, total, err := r.ListEvents(context.Background(), domain.ListEventsQuery{
		OrganizationID: testOrgID,
		EventType:      &eventType,
		Subject:        &subject,
		From:           &fixedFrom,
		To:             &fixedTo,
		Page:           1,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, int64(2), total)
}

// ── InsertEvent ───────────────────────────────────────────────

func TestRepository_InsertEvent_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertEvent)).
		WithArgs(
			"ev-001", testOrgID, "login", "user-123", int64(1), "count",
			"idem-login-001", fixedNow, json.RawMessage(`{"ip":"1.2.3.4"}`),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	ok, err := r.InsertEvent(context.Background(), eventFromRow())
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestRepository_InsertEvent_Duplicate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertEvent)).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 rows affected = 重复

	r := NewRepository(db)
	ok, err := r.InsertEvent(context.Background(), eventFromRow())
	assert.NoError(t, err)
	assert.False(t, ok)
}

// ── IngestBatch ────────────────────────────────────────────────

func TestRepository_IngestBatch(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertEvent)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(queryInsertEvent)).
		WillReturnResult(sqlmock.NewResult(1, 0)) // dup
	mock.ExpectExec(regexp.QuoteMeta(queryInsertEvent)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	evs := []*domain.MeteringEvent{eventFromRow(), eventFromRow(), eventFromRow()}
	ins, dup, err := r.IngestBatch(context.Background(), evs)
	assert.NoError(t, err)
	assert.Equal(t, 2, ins)
	assert.Equal(t, 1, dup)
}

// ── ListAggregates（默认） ─────────────────────────────────────

func TestRepository_ListAggregates_Default(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `metering_aggregates` WHERE organization_id = ?")).
		WithArgs(testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	mock.ExpectQuery(regexp.QuoteMeta("FROM `metering_aggregates` WHERE organization_id = ? ORDER BY period DESC LIMIT ? OFFSET ?")).
		WithArgs(testOrgID, 20, 0).
		WillReturnRows(sqlmock.NewRows(aggCols()).AddRow(newAggRow()...))

	r := NewRepository(db)
	items, total, err := r.ListAggregates(context.Background(), domain.ListAggregatesQuery{
		OrganizationID: testOrgID,
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "agg-001", items[0].ID)
}

// uuidCapture is a sqlmock argument matcher that records the generated id and
// fails the expectation when it is not a syntactically valid UUID.
type uuidCapture struct{ value *string }

func (m uuidCapture) Match(v driver.Value) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	*m.value = s
	return platformuuid.IsValid(s)
}

// ── UpsertAggregate ───────────────────────────────────────────

func TestRepository_UpsertAggregate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpsertAggregate)).
		WithArgs("agg-001", testOrgID, "2026-09", "user-123", "login", int64(42), "count").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.UpsertAggregate(context.Background(), aggFromRow())
	assert.NoError(t, err)
}

// ── RefreshAggregate ───────────────────────────────────────────

func TestRepository_RefreshAggregate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	r := NewRepository(db)
	cmd := domain.RefreshAggregateCommand{
		OrganizationID: testOrgID,
		Subject:        "user-123",
		Metric:         "login",
		Period:         "2026-09",
	}

	// queryRefreshAggregate 参数（15 个）:
	//   1-3: orgID, subject, metric
	//   4-7: PeriodStart, PeriodEnd（WHERE AND 边界分支，全 nil 则走 fallback）
	//   8-10: PeriodStart IS NULL / PeriodEnd IS NULL / DATE_FORMAT fallback
	//   11-15: newID, orgID, period, subject, metric (INSERT)
	var generatedIDs []string
	for i := 0; i < 2; i++ {
		var generatedID string
		mock.ExpectExec(regexp.QuoteMeta(queryRefreshAggregate)).
			WithArgs(
				testOrgID, "user-123", "login",
				nil, nil, nil, nil, nil, nil, "2026-09",
				uuidCapture{&generatedID},
				testOrgID, "2026-09", "user-123", "login",
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// queryRefreshAggregateSelect 参数：orgID, period, subject, metric → 4 个
		mock.ExpectQuery(regexp.QuoteMeta(queryRefreshAggregateSelect)).
			WithArgs(testOrgID, "2026-09", "user-123", "login").
			WillReturnRows(sqlmock.NewRows(aggCols()).AddRow(newAggRow()...))

		agg, err := r.RefreshAggregate(context.Background(), cmd)
		assert.NoError(t, err)
		assert.NotNil(t, agg)
		assert.Equal(t, int64(42), agg.Quantity)
		generatedIDs = append(generatedIDs, generatedID)
	}

	// Replaying the same business key is idempotent through ON DUPLICATE KEY
	// UPDATE, while each attempt gets its own valid UUID primary key.
	assert.True(t, platformuuid.IsValid(generatedIDs[0]), "generated id must be a UUID: %s", generatedIDs[0])
	assert.True(t, platformuuid.IsValid(generatedIDs[1]), "generated id must be a UUID: %s", generatedIDs[1])
	assert.NotEqual(t, generatedIDs[0], generatedIDs[1])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
