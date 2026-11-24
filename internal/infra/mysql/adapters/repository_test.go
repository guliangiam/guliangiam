package adapters

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow        = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testAdapterID   = "00000000-0000-0000-0000-000000000001"
	testOrgID       = "00000000-0000-0000-0000-000000000002"
	testAdapterName = "main-adapter"
	testDisplayName = "Main Adapter"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义 ────────────────────────────────────────────────────────
// 共 24 列，与 scanAdapterRow 的 Scan 参数顺序严格一致。

func adapterCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "description",
		"type", "server", "database", "table_name", "username", "password",
		"owner", "use_same_db", "database_type", "host", "port",
		"db_user", "db_password", "database_name", "policy", "is_enabled",
		"created_at", "updated_at", "revision",
	}
}

// newAdapterRow 严格匹配 scanAdapterRow 的 24 个 Scan 参数顺序。
// password/db_password 在 DB 层存密文（加密开启时），用 adapterEncryptedMapper 动态加密。
func newAdapterRow() []driver.Value {
	encPwd, _ := adapterEncryptedMapper.EncryptColumn(adapterAAD(testAdapterID, adapterColumnPassword), "plain-password")
	encDbPwd, _ := adapterEncryptedMapper.EncryptColumn(adapterAAD(testAdapterID, adapterColumnDbPassword), "db-password")
	port := 5432
	return []driver.Value{
		testAdapterID, testOrgID, testAdapterName, testDisplayName, "Production adapter",
		"database", "localhost", "appdb", "users_table", "admin", encPwd,
		"owner1", true, "postgresql", "db.example.com", &port,
		"dbadmin", encDbPwd, "appdb", "policy=all", true,
		fixedNow, fixedNow, int64(1),
	}
}

func Test_colCounts_Adapters(t *testing.T) {
	t.Logf("adapterCols count: %d", len(adapterCols()))
	t.Logf("adapterRow count:  %d", len(newAdapterRow()))
	assert.Equal(t, len(adapterCols()), len(newAdapterRow()), "cols vs row count mismatch")
}

// ── GetByID ──────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetAdapterByID)).
		WithArgs(testAdapterID, testOrgID).
		WillReturnRows(sqlmock.NewRows(adapterCols()).AddRow(newAdapterRow()...))

	r := NewRepository(db)
	a, err := r.GetByID(context.Background(), testAdapterID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, testAdapterID, a.ID)
	assert.Equal(t, testOrgID, a.OrganizationID)
	assert.Equal(t, testAdapterName, a.Name)
	assert.Equal(t, testDisplayName, a.DisplayName)
	assert.Equal(t, "plain-password", a.Password) // 解密后
	assert.Equal(t, "db-password", a.DbPassword)  // 解密后
	assert.Equal(t, int64(1), a.Revision)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetAdapterByID)).
		WithArgs("unknown-id", testOrgID).
		WillReturnRows(sqlmock.NewRows(adapterCols()))

	r := NewRepository(db)
	a, err := r.GetByID(context.Background(), "unknown-id", testOrgID)
	assert.Error(t, err)
	assert.Nil(t, a)
}

// TestRepository_GetByID_CrossTenantNotFound pins that a lookup scoped to a
// different organization returns no rows and maps to not-found.
func TestRepository_GetByID_CrossTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetAdapterByID)).
		WithArgs(testAdapterID, "other-org").
		WillReturnRows(sqlmock.NewRows(adapterCols()))

	r := NewRepository(db)
	a, err := r.GetByID(context.Background(), testAdapterID, "other-org")
	require.Error(t, err)
	assert.Nil(t, a)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// ── List ─────────────────────────────────────────────────────────
// queryListAdapters 参数：orgID×2, keyword×3, is_enabled×2, database_type×2, limit, offset = 11
// queryCountAdapters 参数：orgID×2, keyword×3, is_enabled×2, database_type×2 = 9

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 默认 ListQuery：空 orgID、空 keyword、nil is_enabled、空 database_type、Page=1、PageSize=20
	mock.ExpectQuery(regexp.QuoteMeta(queryListAdapters)).
		WithArgs("", "", "", "", "", nil, nil, "", "", 20, 0).
		WillReturnRows(sqlmock.NewRows(adapterCols()).AddRow(newAdapterRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountAdapters)).
		WithArgs("", "", "", "", "", nil, nil, "", "").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, testAdapterID, items[0].ID)
}

func TestRepository_List_WithFilter(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	isEnabled := true

	mock.ExpectQuery(regexp.QuoteMeta(queryListAdapters)).
		WithArgs(testOrgID, testOrgID, "main", "main", "main", isEnabled, isEnabled, "postgresql", "postgresql", 20, 0).
		WillReturnRows(sqlmock.NewRows(adapterCols()))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountAdapters)).
		WithArgs(testOrgID, testOrgID, "main", "main", "main", isEnabled, isEnabled, "postgresql", "postgresql").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		Page:           1,
		PageSize:       20,
		OrganizationID: testOrgID,
		Keyword:        "main",
		IsEnabled:      &isEnabled,
		DatabaseType:   "postgresql",
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 24 个 ? 参数
	mock.ExpectExec(regexp.QuoteMeta(queryInsertAdapter)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	port := 5432
	a := &domain.Adapter{
		ID:             testAdapterID,
		OrganizationID: testOrgID,
		Name:           testAdapterName,
		DisplayName:    testDisplayName,
		Type:           "database",
		Username:       "admin",
		Password:       "plain-password",
		DbPassword:     "db-password",
		Port:           &port,
		IsEnabled:      true,
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       1,
	}
	err := r.Create(context.Background(), a)
	assert.NoError(t, err)
}

// ── Update ───────────────────────────────────────────────────────
// queryUpdateAdapter: 22 个 ?（SET 19 + id + revision + organization_id）
// querySelectRevision: 2 个 ?（id + organization_id）

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateAdapter)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testAdapterID, int64(1), testOrgID, // id + revision + organization_id
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevision)).
		WithArgs(testAdapterID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	a := &domain.Adapter{
		ID:             testAdapterID,
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

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateAdapter)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testAdapterID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 rows affected = conflict

	r := NewRepository(db)
	a := &domain.Adapter{
		ID:             testAdapterID,
		OrganizationID: testOrgID,
		DisplayName:    "Updated",
		Revision:       1,
		UpdatedAt:      fixedNow,
	}
	err := r.Update(context.Background(), a)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

// TestRepository_Update_CrossTenantNoRows pins that an update scoped to
// another organization never matches, so the row stays untouched.
func TestRepository_Update_CrossTenantNoRows(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateAdapter)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testAdapterID, int64(1), "other-org",
		).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	a := &domain.Adapter{ID: testAdapterID, OrganizationID: "other-org", Revision: 1, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

// ── Delete ───────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteAdapter)).
		WithArgs(testAdapterID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testAdapterID, testOrgID)
	assert.NoError(t, err)
}

func TestRepository_Delete_CrossTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteAdapter)).
		WithArgs(testAdapterID, "other-org").
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testAdapterID, "other-org")
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
