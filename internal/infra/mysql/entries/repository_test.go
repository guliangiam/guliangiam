package entries

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow2   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID2  = "00000000-0000-0000-0000-000000000001"
	testEntryID = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// getCols: 13 列
func getCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "provider", "application", "type",
		"client_ip", "user_agent", "message", "created_at", "updated_at", "revision",
	}
}

// listCols: 7 列
func listCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "type", "application", "created_at",
	}
}

func newEntryRow() []driver.Value {
	return []driver.Value{
		testEntryID, testOrgID2, "my-entry", "My Entry", "provider-x", "app-y", "login",
		"192.168.1.1", "Mozilla/5.0", "success", fixedNow2, fixedNow2, int64(1),
	}
}

func newListRow() []driver.Value {
	return []driver.Value{
		testEntryID, testOrgID2, "my-entry", "My Entry", "login", "app-y", fixedNow2,
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetEntryByID)).
		WithArgs(testEntryID).
		WillReturnRows(sqlmock.NewRows(getCols()).AddRow(newEntryRow()...))

	r := NewRepository(db)
	e, err := r.GetByID(context.Background(), testEntryID)
	assert.NoError(t, err)
	assert.NotNil(t, e)
	assert.Equal(t, testEntryID, e.ID)
	assert.Equal(t, "provider-x", e.Provider)
	assert.Equal(t, int64(1), e.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetEntryByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(getCols()))

	r := NewRepository(db)
	e, err := r.GetByID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, e)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×4 + type×2 + application×2 = 10
// listArgs: 10 + limit + offset = 12
func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{testOrgID2, testOrgID2, "kw", "kw", "kw", "kw", "login", "login", "app-y", "app-y"}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountEntries)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListEntries)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(listCols()).AddRow(newListRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID2,
		Keyword:        "kw",
		Type:           "login",
		Application:    "app-y",
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_EmptyFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", "", "", "", "", ""}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountEntries)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListEntries)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(listCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertEntry)).
		WithArgs(
			testEntryID, testOrgID2, "my-entry", "My Entry", "provider-x", "app-y",
			"login", "192.168.1.1", "Mozilla/5.0", "success", fixedNow2, fixedNow2, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Entry{
		ID:             testEntryID,
		OrganizationID: testOrgID2,
		Name:           "my-entry",
		DisplayName:    "My Entry",
		Provider:       "provider-x",
		Application:    "app-y",
		Type:           "login",
		ClientIP:       "192.168.1.1",
		UserAgent:      "Mozilla/5.0",
		Message:        "success",
		CreatedAt:      fixedNow2,
		UpdatedAt:      fixedNow2,
		Revision:       int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateEntry)).
		WithArgs("Updated", "done", fixedNow2, int64(2), testEntryID, testOrgID2).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Entry{
		ID:             testEntryID,
		OrganizationID: testOrgID2,
		DisplayName:    "Updated",
		Message:        "done",
		UpdatedAt:      fixedNow2,
		Revision:       int64(2),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteEntry)).
		WithArgs(testEntryID, testOrgID2).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testEntryID, testOrgID2)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
