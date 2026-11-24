package roles

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow     = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testRoleID   = "00000000-0000-0000-0000-000000000001"
	testOrgID    = "00000000-0000-0000-0000-000000000002"
	testRoleName = "admin"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义 ────────────────────────────────────────────────────────
// 所有列的数量必须与 repository.go 里 scanRoleRow 的 Scan 参数数量严格一致（13 列）。

func roleCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "description",
		"users", "group_ids", "role_ids", "domains",
		"is_enabled", "created_at", "updated_at", "revision",
	}
}

// ── 行数据构造 ────────────────────────────────────────────────────

// newRoleRow 严格匹配 scanRoleRow 的 13 个 Scan 参数顺序。
func newRoleRow() []driver.Value {
	return []driver.Value{
		testRoleID, testOrgID, testRoleName, "Administrator", "Full administrative access",
		[]byte(`["user-001","user-002"]`),
		[]byte(`["group-001"]`),
		[]byte(`["role-super"]`),
		[]byte(`["domain-001"]`),
		true,
		fixedNow, fixedNow, int64(1),
	}
}

// newEmptyRoleRow 四个 JSON 列都是空数组，用于边界测试。
func newEmptyRoleRow() []driver.Value {
	return []driver.Value{
		testRoleID, testOrgID, testRoleName, "Administrator", "",
		[]byte(`[]`), []byte(`[]`), []byte(`[]`), []byte(`[]`),
		false,
		fixedNow, fixedNow, int64(1),
	}
}

// ── Test 列数（先跑这个验证列数正确）──────────────────────────────

func Test_colCounts_Roles(t *testing.T) {
	t.Logf("roleCols count: %d", len(roleCols()))
	t.Logf("roleRow count:  %d", len(newRoleRow()))
	assert.Equal(t, len(roleCols()), len(newRoleRow()), "cols vs row count mismatch")
}

// ── GetByID ──────────────────────────────────────────────────────

func TestRepository_GetByIDForOrganization_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetRoleByIDForOrganization)).
		WithArgs(testRoleID, testOrgID).
		WillReturnRows(sqlmock.NewRows(roleCols()).AddRow(newRoleRow()...))

	r := NewRepository(db)
	role, err := r.GetByIDForOrganization(context.Background(), testRoleID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, testRoleID, role.ID)
	assert.Equal(t, testOrgID, role.OrganizationID)
	assert.Equal(t, testRoleName, role.Name)
	assert.Equal(t, "Administrator", role.DisplayName)
	assert.Equal(t, []string{"user-001", "user-002"}, role.Users)
	assert.Equal(t, []string{"group-001"}, role.GroupIDs)
	assert.Equal(t, []string{"role-super"}, role.RoleIDs)
	assert.Equal(t, []string{"domain-001"}, role.Domains)
	assert.True(t, role.IsEnabled)
	assert.Equal(t, int64(1), role.Revision)
}

func TestRepository_GetByIDForOrganization_ForeignTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetRoleByIDForOrganization)).
		WithArgs(testRoleID, "other-org").
		WillReturnRows(sqlmock.NewRows(roleCols()))

	r := NewRepository(db)
	role, err := r.GetByIDForOrganization(context.Background(), testRoleID, "other-org")
	assert.Error(t, err)
	assert.Nil(t, role)
}

// ── List ─────────────────────────────────────────────────────────
// queryListRoles 参数顺序：organization_id(×2), keyword(×4), is_enabled(×2), limit, offset = 10
// queryCountRoles 参数顺序：organization_id(×2), keyword(×4), is_enabled(×2) = 8

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 默认 ListQuery：空 orgID、空 keyword、nil is_enabled、Page=1、PageSize=20
	// orgID × 2, keyword × 4, is_enabled × 2, limit, offset = 10
	// count: orgID × 2, keyword × 4, is_enabled × 2 = 8
	mock.ExpectQuery(regexp.QuoteMeta(queryListRoles)).
		WithArgs("", "", "", "", "", "", nil, nil, 20, 0).
		WillReturnRows(sqlmock.NewRows(roleCols()).AddRow(newRoleRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountRoles)).
		WithArgs("", "", "", "", "", "", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, testRoleID, items[0].ID)
}

func TestRepository_List_WithFilter(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	isEnabled := true

	mock.ExpectQuery(regexp.QuoteMeta(queryListRoles)).
		WithArgs(testOrgID, testOrgID, "admin", "admin", "admin", "admin", isEnabled, isEnabled, 20, 0).
		WillReturnRows(sqlmock.NewRows(roleCols()))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountRoles)).
		WithArgs(testOrgID, testOrgID, "admin", "admin", "admin", "admin", isEnabled, isEnabled).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		Page:           1,
		PageSize:       20,
		OrganizationID: testOrgID,
		Keyword:        "admin",
		IsEnabled:      &isEnabled,
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestRepository_List_EmptyJSON(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListRoles)).
		WithArgs("", "", "", "", "", "", nil, nil, 20, 0).
		WillReturnRows(sqlmock.NewRows(roleCols()).AddRow(newEmptyRoleRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountRoles)).
		WithArgs("", "", "", "", "", "", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	role := items[0]
	assert.Equal(t, []string{}, role.Users)
	assert.Equal(t, []string{}, role.GroupIDs)
	assert.Equal(t, []string{}, role.RoleIDs)
	assert.Equal(t, []string{}, role.Domains)
	assert.False(t, role.IsEnabled)
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 13 个 ? 参数
	mock.ExpectExec(regexp.QuoteMeta(queryInsertRole)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	role := &domain.Role{
		ID:             testRoleID,
		OrganizationID: testOrgID,
		Name:           testRoleName,
		DisplayName:    "Administrator",
		IsEnabled:      true,
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       1,
	}
	err := r.Create(context.Background(), role)
	assert.NoError(t, err)
}

// ── Update ───────────────────────────────────────────────────────
// queryUpdateRole: 11 个 ?（display_name, description, users, group_ids, role_ids, domains, is_enabled, updated_at, id, old_revision, organization_id）
// querySelectRevision: 2 个 ?（id, organization_id）

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateRole)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			testRoleID, int64(1), testOrgID, // id + old_revision（role.Revision-1 = 2-1 = 1）+ organization_id
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevision)).
		WithArgs(testRoleID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	role := &domain.Role{
		ID:             testRoleID,
		OrganizationID: testOrgID,
		DisplayName:    "Updated Admin",
		Description:    "Updated",
		Revision:       2,
		UpdatedAt:      fixedNow,
	}
	err := r.Update(context.Background(), role)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), role.Revision)
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateRole)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			testRoleID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 rows affected = conflict

	r := NewRepository(db)
	role := &domain.Role{
		ID:             testRoleID,
		OrganizationID: testOrgID,
		DisplayName:    "Updated",
		Revision:       2,
		UpdatedAt:      fixedNow,
	}
	err := r.Update(context.Background(), role)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

// ── Delete ───────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteRole)).
		WithArgs(testRoleID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testRoleID, testOrgID)
	assert.NoError(t, err)
}

func TestRepository_Delete_ForeignTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteRole)).
		WithArgs(testRoleID, "other-org").
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testRoleID, "other-org")
	assert.Error(t, err)
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
