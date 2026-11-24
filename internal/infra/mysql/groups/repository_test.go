package groups

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow     = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID    = "00000000-0000-0000-0000-000000000001"
	testGroupID  = "00000000-0000-0000-0000-000000000002"
	testParentID = "00000000-0000-0000-0000-000000000003"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// groupCols: 15 列
func groupCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "manager", "contact_email", "type", "parent_id",
		"is_top_group", "is_enabled", "homepage", "users",
		"created_at", "updated_at", "revision",
	}
}

func newGroupRow() []driver.Value {
	usersJSON, _ := json.Marshal([]string{"user1", "user2"})
	return []driver.Value{
		testGroupID, testOrgID, "Engineering", "Engineering Dept",
		"manager-id", "mgr@example.com", "department", testParentID,
		false, true, "https://eng.example.com", string(usersJSON),
		fixedNow, fixedNow, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetGroupByID)).
		WithArgs(testGroupID).
		WillReturnRows(sqlmock.NewRows(groupCols()).AddRow(newGroupRow()...))

	r := NewRepository(db)
	g, err := r.GetByID(context.Background(), testGroupID)
	assert.NoError(t, err)
	assert.NotNil(t, g)
	assert.Equal(t, testGroupID, g.ID)
	assert.Equal(t, "Engineering", g.Name)
	assert.Equal(t, []string{"user1", "user2"}, g.Users)
	assert.Equal(t, int64(1), g.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetGroupByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(groupCols()))

	r := NewRepository(db)
	g, err := r.GetByID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, g)
}

func TestRepository_GetByIDForOrganization_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetGroupByIDForOrganization)).
		WithArgs(testGroupID, testOrgID).
		WillReturnRows(sqlmock.NewRows(groupCols()).AddRow(newGroupRow()...))

	r := NewRepository(db)
	g, err := r.GetByIDForOrganization(context.Background(), testGroupID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, g)
	assert.Equal(t, testGroupID, g.ID)
	assert.Equal(t, testOrgID, g.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_ForeignTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetGroupByIDForOrganization)).
		WithArgs(testGroupID, "other-org").
		WillReturnRows(sqlmock.NewRows(groupCols()))

	r := NewRepository(db)
	g, err := r.GetByIDForOrganization(context.Background(), testGroupID, "other-org")
	assert.Error(t, err)
	assert.Nil(t, g)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org×2 + keyword×3 + status×2 + type×2 = 9
// listArgs: countArgs + 20 + 0 = 11
func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{
		testOrgID, testOrgID,
		"eng", "eng", "eng",
		"active", "active",
		"department", "department",
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountGroups)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListGroups)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(groupCols()).AddRow(newGroupRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID,
		Keyword:        "eng",
		Status:         "active",
		Type:           "department",
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

	countArgs := []driver.Value{"", "", "", "", "", "", "", "", ""}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountGroups)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListGroups)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(groupCols()))

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

	usersJSON, _ := json.Marshal([]string{"user1", "user2"})
	mock.ExpectExec(regexp.QuoteMeta(queryInsertGroup)).
		WithArgs(
			testGroupID, testOrgID, "Engineering", "Engineering Dept",
			"manager-id", "mgr@example.com", "department", testParentID,
			false, true, "https://eng.example.com", string(usersJSON),
			fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Group{
		ID:             testGroupID,
		OrganizationID: testOrgID,
		Name:           "Engineering",
		DisplayName:    "Engineering Dept",
		Manager:        "manager-id",
		ContactEmail:   "mgr@example.com",
		Type:           "department",
		ParentID:       testParentID,
		IsEnabled:      true,
		Homepage:       "https://eng.example.com",
		Users:          []string{"user1", "user2"},
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	usersJSON, _ := json.Marshal([]string{"user1", "user2", "user3"})
	// UPDATE: name(1), display_name(1), manager(1), contact_email(1), type(1), parent_id(1),
	//   is_top_group(1), is_enabled(1), homepage(1), users(1), updated_at(1), id(1), revision(旧)(1), organization_id(1)
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateGroup)).
		WithArgs(
			"Engineering Updated", "Eng Dept Updated",
			"new-mgr", "new-mgr@example.com", "team", testParentID,
			true, true, "https://new-eng.example.com", string(usersJSON),
			fixedNow, testGroupID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectGroupRevision)).
		WithArgs(testGroupID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	g := &domain.Group{
		ID:             testGroupID,
		OrganizationID: testOrgID,
		Name:           "Engineering Updated",
		DisplayName:    "Eng Dept Updated",
		Manager:        "new-mgr",
		ContactEmail:   "new-mgr@example.com",
		Type:           "team",
		ParentID:       testParentID,
		IsTopGroup:     true,
		IsEnabled:      true,
		Homepage:       "https://new-eng.example.com",
		Users:          []string{"user1", "user2", "user3"},
		UpdatedAt:      fixedNow,
		Revision:       int64(1),
	}
	err := r.Update(context.Background(), g)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), g.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	usersJSON, _ := json.Marshal([]string{})
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateGroup)).
		WithArgs(
			"n", "d", "m", "e", "t", "p", false, false, "", string(usersJSON),
			fixedNow, testGroupID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 affected → conflict

	r := NewRepository(db)
	g := &domain.Group{
		ID:             testGroupID,
		OrganizationID: testOrgID,
		Name:           "n",
		DisplayName:    "d",
		Manager:        "m",
		ContactEmail:   "e",
		Type:           "t",
		ParentID:       "p",
		UpdatedAt:      fixedNow,
		Revision:       int64(1),
	}
	err := r.Update(context.Background(), g)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteGroup)).
		WithArgs(testGroupID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testGroupID, testOrgID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteGroup)).
		WithArgs("bad", testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 affected → not found

	r := NewRepository(db)
	err := r.Delete(context.Background(), "bad", testOrgID)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── CountChildrenByParentID ───────────────────────────────────────

func TestRepository_CountChildrenByParentID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryCountChildrenByParentID)).
		WithArgs(testOrgID, testParentID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(5)))

	r := NewRepository(db)
	n, err := r.CountChildrenByParentID(context.Background(), testOrgID, testParentID)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
