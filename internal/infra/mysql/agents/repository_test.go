package agents

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/agents"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

var (
	fixedNow        = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testAgentID     = "00000000-0000-0000-0000-000000000001"
	testOrgID       = "00000000-0000-0000-0000-000000000002"
	testAgentName   = "node-agent-1"
	testDisplayName = "Node Agent 1"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

func agentCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "type",
		"host", "port", "url", "token", "application_id",
		"is_enabled", "last_ping_at", "created_at", "updated_at", "revision",
	}
}

func newAgentRow() []driver.Value {
	encToken, _ := agentEncryptedMapper.EncryptColumn(agentAAD(testAgentID), "s3cret-token")
	return []driver.Value{
		testAgentID, testOrgID, testAgentName, testDisplayName, "node",
		"agent.example.com", 8080, "https://agent.example.com", encToken, "app-001",
		true, (*time.Time)(nil), fixedNow, fixedNow, int64(1),
	}
}

func Test_colCounts_Agents(t *testing.T) {
	t.Logf("agentCols count: %d", len(agentCols()))
	t.Logf("agentRow count:  %d", len(newAgentRow()))
	assert.Equal(t, len(agentCols()), len(newAgentRow()), "cols vs row count mismatch")
}

// ── GetByID ──────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetAgentByID)).
		WithArgs(testAgentID, testOrgID).
		WillReturnRows(sqlmock.NewRows(agentCols()).AddRow(newAgentRow()...))

	r := NewRepository(db)
	a, err := r.GetByID(context.Background(), testAgentID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, testAgentID, a.ID)
	assert.Equal(t, testOrgID, a.OrganizationID)
	assert.Equal(t, testAgentName, a.Name)
	assert.Equal(t, "s3cret-token", a.Token) // 解密后
	assert.Equal(t, int64(1), a.Revision)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetAgentByID)).
		WithArgs("unknown-id", testOrgID).
		WillReturnRows(sqlmock.NewRows(agentCols()))

	r := NewRepository(db)
	a, err := r.GetByID(context.Background(), "unknown-id", testOrgID)
	assert.Error(t, err)
	assert.Nil(t, a)
}

func TestRepository_GetByID_ForeignOrganizationNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetAgentByID)).
		WithArgs(testAgentID, "foreign-org").
		WillReturnRows(sqlmock.NewRows(agentCols()))

	r := NewRepository(db)
	a, err := r.GetByID(context.Background(), testAgentID, "foreign-org")
	assert.Error(t, err)
	assert.Nil(t, a)
}

// ── List ─────────────────────────────────────────────────────────

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListAgents)).
		WithArgs("", "", "", "", "", nil, nil, 20, 0).
		WillReturnRows(sqlmock.NewRows(agentCols()).AddRow(newAgentRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountAgents)).
		WithArgs("", "", "", "", "", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
}

func TestRepository_List_WithFilter(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	isEnabled := true

	mock.ExpectQuery(regexp.QuoteMeta(queryListAgents)).
		WithArgs(testOrgID, testOrgID, "node", "node", "node", isEnabled, isEnabled, 20, 0).
		WillReturnRows(sqlmock.NewRows(agentCols()))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountAgents)).
		WithArgs(testOrgID, testOrgID, "node", "node", "node", isEnabled, isEnabled).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		Page:           1,
		PageSize:       20,
		OrganizationID: testOrgID,
		Keyword:        "node",
		IsEnabled:      &isEnabled,
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertAgent)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	a := &domain.Agent{
		ID:             testAgentID,
		OrganizationID: testOrgID,
		Name:           testAgentName,
		DisplayName:    testDisplayName,
		Type:           "node",
		Port:           8080,
		Token:          "s3cret-token",
		IsEnabled:      true,
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       1,
	}
	err := r.Create(context.Background(), a)
	assert.NoError(t, err)
}

// ── Update ───────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateAgent)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testAgentID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevision)).
		WithArgs(testAgentID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	a := &domain.Agent{
		ID:             testAgentID,
		OrganizationID: testOrgID,
		DisplayName:    "Updated Display",
		Revision:       1,
		UpdatedAt:      fixedNow,
	}
	err := r.Update(context.Background(), a)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), a.Revision)
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateAgent)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testAgentID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	a := &domain.Agent{ID: testAgentID, OrganizationID: testOrgID, DisplayName: "Updated", Revision: 1, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), a)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

// ── Delete ───────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteAgent)).
		WithArgs(testAgentID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testAgentID, testOrgID)
	assert.NoError(t, err)
}

func TestRepository_Delete_ForeignOrganizationNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteAgent)).
		WithArgs(testAgentID, "foreign-org").
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testAgentID, "foreign-org")
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestRepository_ByIDQueriesCarryTenantPredicate pins the SQL text itself: on
// MySQL there is no RLS, so an id-only predicate is the IDOR surface.
func TestRepository_ByIDQueriesCarryTenantPredicate(t *testing.T) {
	assert.Contains(t, queryGetAgentByID, "WHERE id = ? AND organization_id = ?")
	assert.Contains(t, queryUpdateAgent, "WHERE id = ? AND revision = ? AND organization_id = ?")
	assert.Contains(t, querySelectRevision, "WHERE id = ? AND organization_id = ?")
	assert.Contains(t, queryDeleteAgent, "WHERE id = ? AND organization_id = ?")
}

var _ domain.Repository = Repository{}
