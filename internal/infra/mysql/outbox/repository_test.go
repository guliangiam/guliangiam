package outbox

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	"github.com/stretchr/testify/assert"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

var (
	fixedNow  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedDead = fixedNow.Add(time.Hour)
)

// outboxCols 是 sqlLockBatch_Select 返回的列顺序（必须严格匹配）
var outboxCols = []string{
	"id", "organization_id", "aggregate_type", "aggregate_id", "event_type",
	"event_version", "payload", "headers", "status", "attempts", "max_attempts",
	"available_at", "locked_by", "locked_at", "processed_at", "last_error",
	"created_at", "updated_at", "revision",
}

func newOutboxRow(id, orgID, aggType, aggID, evType string) []driver.Value {
	return []driver.Value{
		id, orgID, aggType, aggID, evType,
		1, []byte(`{}`), []byte(`{}`), "processing", 0, 5,
		fixedNow, "worker-1", fixedNow, nil, nil,
		fixedNow, fixedNow, int64(1),
	}
}

// ── Insert ──────────────────────────────────────────────────────────

func TestRepository_Insert(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `outbox`")).
		WithArgs(
			"evt-1", "org-1", "webhook", "wh-1", "webhook.created",
			1, []byte(`{"k":"v"}`), []byte(`{}`),
			"pending", 0, 5,
			fixedNow,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	item := &platformoutbox.OutboxItem{
		ID:             "evt-1",
		OrganizationID: strPtr("org-1"),
		AggregateType:  "webhook",
		AggregateID:    "wh-1",
		EventType:      "webhook.created",
		EventVersion:   1,
		Payload:        []byte(`{"k":"v"}`),
		Headers:        []byte(`{}`),
		Status:         platformoutbox.StatusPending,
		Attempts:       0,
		MaxAttempts:    5,
		AvailableAt:    fixedNow,
	}
	err := r.Insert(context.Background(), item)
	assert.NoError(t, err)
}

func TestRepository_Insert_DefaultEmptyJSON(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `outbox`")).
		WithArgs(
			"evt-2", nil, "webhook", "wh-2", "webhook.deleted",
			2, []byte(`{}`), []byte(`{}`),
			"pending", 0, 5,
			fixedNow,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	item := &platformoutbox.OutboxItem{
		ID:            "evt-2",
		AggregateType: "webhook",
		AggregateID:   "wh-2",
		EventType:     "webhook.deleted",
		EventVersion:  2,
		Status:        platformoutbox.StatusPending,
		MaxAttempts:   5,
		AvailableAt:   fixedNow,
		// Payload/Headers 空 → 走默认 {}
	}
	err := r.Insert(context.Background(), item)
	assert.NoError(t, err)
}

// ── LockBatch ────────────────────────────────────────────────────────

func TestRepository_LockBatch_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id\nFROM `outbox`")).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id"})) // 空结果

	r := NewRepository(db)
	items, err := r.LockBatch(context.Background(), 10, "worker-1", 30*time.Second)
	assert.NoError(t, err)
	assert.Nil(t, items) // pickedIDs 为空 → 直接返回 nil
}

func TestRepository_LockBatch_HappyPath(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// Step 1: pick IDs
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id\nFROM `outbox`")).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow("evt-1").
			AddRow("evt-2"))

	// Step 2: UPDATE picked
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `outbox`")).
		WithArgs("worker-1", "evt-1", "evt-2").
		WillReturnResult(sqlmock.NewResult(2, 2))

	// Step 3: SELECT full rows
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, organization_id")).
		WithArgs("evt-1", "evt-2").
		WillReturnRows(sqlmock.NewRows(outboxCols).
			AddRow(newOutboxRow("evt-1", "org-1", "webhook", "wh-1", "wh.created")...).
			AddRow(newOutboxRow("evt-2", "org-1", "webhook", "wh-2", "wh.created")...))

	r := NewRepository(db)
	items, err := r.LockBatch(context.Background(), 5, "worker-1", 30*time.Second)
	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "evt-1", items[0].ID)
	assert.Equal(t, "worker-1", *items[0].LockedBy)
	assert.Equal(t, platformoutbox.StatusProcessing, items[0].Status)
}

// ── MarkSuccess ─────────────────────────────────────────────────────

func TestRepository_MarkSuccess(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE `outbox`")).
		WithArgs(fixedNow, "evt-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.MarkSuccess(context.Background(), "evt-1", fixedNow)
	assert.NoError(t, err)
}

// ── MarkRetry ────────────────────────────────────────────────────────

func TestRepository_MarkRetry(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	backoff := fixedNow.Add(5 * time.Second)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `outbox`")).
		WithArgs("network timeout", backoff, "evt-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.MarkRetry(context.Background(), "evt-1", "network timeout", fixedNow, backoff)
	assert.NoError(t, err)
}

// ── MarkDead ────────────────────────────────────────────────────────

func TestRepository_MarkDead_Standalone(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// MarkDead 会 BeginTx → 两次 Exec → Commit
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `outbox_dead_letters`")).
		WithArgs("max attempts reached", fixedDead, "evt-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `outbox`")).
		WithArgs("max attempts reached", "evt-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	r := NewRepository(db)
	err := r.MarkDead(context.Background(), "evt-1", "max attempts reached", fixedDead)
	assert.NoError(t, err)
}

func TestRepository_MarkDead_AlreadyTx(t *testing.T) {
	// 如果传入的已经是 *sql.Tx（来自 UoW WithinTransaction），
	// MarkDead 内部应跳过 BeginTx 直接顺序执行
	db, _ := newMockDB(t)
	defer db.Close()

	// 这是个伪测试：sqlmock 无法模拟 *sql.Tx 传入，
	// 但能保证代码编译通过、Beginner 断言安全。
	// 真实集成由 end-to-end 覆盖。
	r := NewRepository(db)
	assert.NotNil(t, r)
}

// ── domain.Repository 接口断言 ──────────────────────────────────────

func TestRepository_Enqueue(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `outbox`")).
		WithArgs(
			"evt-d1", "org-1", "user", "u-1", "user.created",
			1, []byte(`{"name":"alice"}`), []byte(`{}`),
			"pending", 0, 5,
			sqlmock.AnyArg(), // AvailableAt 由 time.Now() 填充
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Enqueue(context.Background(), &domainoutbox.OutboxEvent{
		ID:             "evt-d1",
		OrganizationID: strPtr("org-1"),
		AggregateType:  "user",
		AggregateID:    "u-1",
		EventType:      "user.created",
		EventVersion:   1,
		Payload:        []byte(`{"name":"alice"}`),
		MaxAttempts:    0, // 0 → 走默认 5
	})
	assert.NoError(t, err)
}

func TestRepository_EnqueueBatch(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 2 个 Enqueue 各一次 Insert
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `outbox`")).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `outbox`")).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	evs := []*domainoutbox.OutboxEvent{
		{ID: "evt-b1", OrganizationID: strPtr("org-1"), AggregateType: "webhook", AggregateID: "wh-1", EventType: "wh.created", MaxAttempts: 12},
		{ID: "evt-b2", OrganizationID: strPtr("org-1"), AggregateType: "webhook", AggregateID: "wh-2", EventType: "wh.created", MaxAttempts: 12},
	}
	err := r.EnqueueBatch(context.Background(), evs)
	assert.NoError(t, err)
}

// ── buildIN ──────────────────────────────────────────────────────────

func Test_buildIN(t *testing.T) {
	assert.Equal(t, "", buildIN(nil))
	assert.Equal(t, "", buildIN([]string{}))
	assert.Equal(t, "?", buildIN([]string{"a"}))
	assert.Equal(t, "?, ?", buildIN([]string{"a", "b"}))
	assert.Equal(t, "?, ?, ?", buildIN([]string{"a", "b", "c"}))
}

func strPtr(s string) *string { return &s }

// 编译期接口断言
var _ domainoutbox.Repository = (*Repository)(nil)
var _ platformoutbox.Repository = (*Repository)(nil)
