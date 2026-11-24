package actions

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/actions"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID = "00000000-0000-0000-0000-000000000001"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── Actions 列定义（8 列） ────────────────────────────────────

func actionCols() []string {
	return []string{
		"id", "organization_id", "name", "trigger_type", "status",
		"created_at", "updated_at", "revision",
	}
}

func newActionRow() []driver.Value {
	return []driver.Value{
		"act-001", testOrgID, "login-check", "pre_login", "active",
		fixedNow, fixedNow, int64(1),
	}
}

// ── Versions 列定义（5 列） ──────────────────────────────────

func versionCols() []string {
	return []string{"id", "action_id", "version", "code", "created_at"}
}

func newVersionRow() []driver.Value {
	return []driver.Value{
		"ver-001", "act-001", 1, "return ctx;", fixedNow,
	}
}

// ── ExecLogs 列定义（9 列） ───────────────────────────────────

func execLogCols() []string {
	return []string{
		"id", "action_id", "version", "status", "input", "output", "error", "executed_at", "created_at",
	}
}

func newExecLogRow() []driver.Value {
	errMsg := "timeout"
	return []driver.Value{
		"log-001", "act-001", 1, "failed",
		`{"user":"u1"}`, `{"result":null}`, &errMsg,
		fixedNow, fixedNow,
	}
}

func newExecLogRowNoOutput() []driver.Value {
	return []driver.Value{
		"log-002", "act-001", 2, "pending",
		`{"user":"u1"}`, "", nil,
		nil, fixedNow,
	}
}

// ── Actions: List ─────────────────────────────────────────────

func TestRepository_List_Default(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `identity_actions` WHERE organization_id = ?")).
		WithArgs(testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	mock.ExpectQuery(regexp.QuoteMeta("FROM `identity_actions` WHERE organization_id = ?")).
		WithArgs(testOrgID, 20, 0).
		WillReturnRows(sqlmock.NewRows(actionCols()).AddRow(newActionRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListActionsQuery{
		OrganizationID: testOrgID, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "act-001", items[0].ID)
	assert.Equal(t, domain.TriggerPreLogin, items[0].TriggerType)
	assert.Equal(t, domain.ActionStatusActive, items[0].Status)
}

func TestRepository_List_WithFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	tt := domain.TriggerPreLogin
	st := domain.ActionStatusActive

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `identity_actions` WHERE organization_id = ?")).
		WithArgs(testOrgID, string(tt), string(st)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	mock.ExpectQuery(regexp.QuoteMeta("FROM `identity_actions` WHERE organization_id = ?")).
		WithArgs(testOrgID, string(tt), string(st), 10, 0).
		WillReturnRows(sqlmock.NewRows(actionCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListActionsQuery{
		OrganizationID: testOrgID,
		TriggerType:    &tt,
		Status:         &st,
		Page:           1, PageSize: 10,
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(1), total)
}

// ── Actions: Get ────────────────────────────────────────────────

func TestRepository_Get(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetAction)).
		WithArgs("act-001", testOrgID).
		WillReturnRows(sqlmock.NewRows(actionCols()).AddRow(newActionRow()...))

	r := NewRepository(db)
	a, err := r.Get(context.Background(), "act-001", testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, "act-001", a.ID)
	assert.Equal(t, domain.TriggerPreLogin, a.TriggerType)
}

// ── Actions: Create ─────────────────────────────────────────────

func TestRepository_Create_WithVersion(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertAction)).
		WithArgs("act-001", testOrgID, "login-check", "pre_login", "active").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta(queryInsertVersion)).
		WithArgs("ver-001", "act-001", 1, "return ctx;").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(),
		&domain.Action{
			ID: "act-001", OrganizationID: testOrgID, Name: "login-check",
			TriggerType: domain.TriggerPreLogin, Status: domain.ActionStatusActive,
		},
		&domain.ActionVersion{
			ID: "ver-001", ActionID: "act-001", Version: 1, Code: "return ctx;",
		},
	)
	assert.NoError(t, err)
}

func TestRepository_Create_ActionOnly(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertAction)).
		WithArgs("act-001", testOrgID, "login-check", "pre_login", "draft").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(),
		&domain.Action{
			ID: "act-001", OrganizationID: testOrgID, Name: "login-check",
			TriggerType: domain.TriggerPreLogin, Status: domain.ActionStatusDraft,
		},
		nil,
	)
	assert.NoError(t, err)
}

// ── Actions: UpdateStatus ──────────────────────────────────────

func TestRepository_UpdateStatus(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateActionStatus)).
		WithArgs("disabled", "act-001", testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.UpdateStatus(context.Background(), domain.UpdateStatusCommand{
		ActionID:       "act-001",
		OrganizationID: testOrgID,
		Status:         domain.ActionStatusDisabled,
	})
	assert.NoError(t, err)
}

// ── Versions: ListVersions ──────────────────────────────────────

func TestRepository_ListVersions(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `action_versions` WHERE action_id = ? AND EXISTS (SELECT 1 FROM `identity_actions` a WHERE a.id = action_versions.action_id AND a.organization_id = ?)")).
		WithArgs("act-001", testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	mock.ExpectQuery(regexp.QuoteMeta("FROM `action_versions` WHERE action_id = ? AND EXISTS (SELECT 1 FROM `identity_actions` a WHERE a.id = action_versions.action_id AND a.organization_id = ?) ORDER BY version DESC LIMIT ? OFFSET ?")).
		WithArgs("act-001", testOrgID, 20, 0).
		WillReturnRows(sqlmock.NewRows(versionCols()).AddRow(newVersionRow()...))

	r := NewRepository(db)
	items, total, err := r.ListVersions(context.Background(), domain.ListActionVersionsQuery{
		OrganizationID: testOrgID, ActionID: "act-001", Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "ver-001", items[0].ID)
}

// ── Versions: PublishVersion ────────────────────────────────────

func TestRepository_PublishVersion(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertVersion)).
		WithArgs("ver-002", "act-001", 2, "return ctx + updated;").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.PublishVersion(context.Background(), "act-001", testOrgID, &domain.ActionVersion{
		ID: "ver-002", ActionID: "act-001", Version: 2, Code: "return ctx + updated;",
	})
	assert.NoError(t, err)
}

// ── Versions: GetLatestVersion ───────────────────────────────────

func TestRepository_GetLatestVersion(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryLatestVersion)).
		WithArgs("act-001").
		WillReturnRows(sqlmock.NewRows(versionCols()).AddRow(newVersionRow()...))

	r := NewRepository(db)
	v, err := r.GetLatestVersion(context.Background(), "act-001", testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, v)
	assert.Equal(t, 1, v.Version)
	assert.Equal(t, "return ctx;", v.Code)
}

// ── ExecutionLogs: ListExecutionLogs ──────────────────────────────

func TestRepository_ListExecutionLogs_Default(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `execution_logs` WHERE action_id = ? AND EXISTS (SELECT 1 FROM `identity_actions` a WHERE a.id = execution_logs.action_id AND a.organization_id = ?)")).
		WithArgs("act-001", testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(2)))

	mock.ExpectQuery(regexp.QuoteMeta("FROM `execution_logs` WHERE action_id = ? AND EXISTS (SELECT 1 FROM `identity_actions` a WHERE a.id = execution_logs.action_id AND a.organization_id = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?")).
		WithArgs("act-001", testOrgID, 20, 0).
		WillReturnRows(sqlmock.NewRows(execLogCols()).AddRow(newExecLogRow()...).AddRow(newExecLogRowNoOutput()...))

	r := NewRepository(db)
	items, total, err := r.ListExecutionLogs(context.Background(), domain.ListExecutionLogsQuery{
		OrganizationID: testOrgID, ActionID: "act-001", Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, domain.ExecStatusFailed, items[0].Status)
	assert.NotNil(t, items[0].Error)
	assert.Equal(t, "timeout", *items[0].Error)
	assert.NotNil(t, items[0].ExecutedAt)

	// 第二个行：pending，无 output/executed_at
	assert.Equal(t, domain.ExecStatusPending, items[1].Status)
	assert.Nil(t, items[1].Error)
	assert.Nil(t, items[1].ExecutedAt)
}

func TestRepository_ListExecutionLogs_WithStatus(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	st := domain.ExecStatusSuccess

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `execution_logs` WHERE action_id = ? AND EXISTS (SELECT 1 FROM `identity_actions` a WHERE a.id = execution_logs.action_id AND a.organization_id = ?) AND status = ?")).
		WithArgs("act-001", testOrgID, string(st)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	mock.ExpectQuery(regexp.QuoteMeta("FROM `execution_logs` WHERE action_id = ? AND EXISTS (SELECT 1 FROM `identity_actions` a WHERE a.id = execution_logs.action_id AND a.organization_id = ?) AND status = ? ORDER BY created_at DESC LIMIT ? OFFSET ?")).
		WithArgs("act-001", testOrgID, string(st), 20, 0).
		WillReturnRows(sqlmock.NewRows(execLogCols()))

	r := NewRepository(db)
	items, total, err := r.ListExecutionLogs(context.Background(), domain.ListExecutionLogsQuery{
		OrganizationID: testOrgID, ActionID: "act-001", Status: &st, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

// ── ExecutionLogs: CreateExecutionLog ────────────────────────────

func TestRepository_CreateExecutionLog(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertExecLog)).
		WithArgs("log-001", "act-001", 1, `{"user":"u1"}`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.CreateExecutionLog(context.Background(), &domain.ExecutionLog{
		ID:       "log-001",
		ActionID: "act-001",
		Version:  1,
		Input:    json.RawMessage(`{"user":"u1"}`),
	})
	assert.NoError(t, err)
}

// ── ExecutionLogs: UpdateExecutionLog ────────────────────────────

func TestRepository_UpdateExecutionLog(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	errMsg := "timeout"
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateExecLog)).
		WithArgs("failed", `{"result":null}`, &errMsg, "log-001").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.UpdateExecutionLog(context.Background(), domain.UpdateExecutionLogCommand{
		LogID:  "log-001",
		Status: domain.ExecStatusFailed,
		Output: json.RawMessage(`{"result":null}`),
		Error:  &errMsg,
	})
	assert.NoError(t, err)
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
