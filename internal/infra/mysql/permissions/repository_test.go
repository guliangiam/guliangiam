package permissions

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow         = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testPermID       = "00000000-0000-0000-0000-000000000001"
	testOrgID        = "00000000-0000-0000-0000-000000000002"
	testPermName     = "test-permission"
	testPermDispName = "Test Permission"
	testPermDesc     = "A test permission"
	testPermResType  = "resource"
	testPermEffect   = "Allow"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义（与 permissionRow 扫描顺序严格一致，共 25 列）────────

func permCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "description",
		"users", "groups", "roles", "domains",
		"resource_type", "resources", "actions", "effect",
		"is_enabled", "submitter", "approver", "state",
		"model", "adapter", "sorter", "application",
		"approve_time", "created_at", "updated_at", "revision",
	}
}

// ── 行数据构造（严格匹配 scanPermissionRow 的 25 个 Scan 参数）───

func newPermRow() []driver.Value {
	row := make([]driver.Value, 0, 25)
	row = append(row,
		testPermID, testOrgID, testPermName, testPermDispName, testPermDesc,
		[]byte(`["user1","user2"]`),
		[]byte(`[]`),
		[]byte(`["admin"]`),
		[]byte(`["domain1"]`),
		testPermResType,
		[]byte(`["res1","res2"]`),
		[]byte(`["read","write"]`),
		testPermEffect,
		true,
		"submitter1", "approver1", "pending",
		"casbin", "gorm", "default", "app1",
		nil, fixedNow, fixedNow, int64(1),
	)
	return row
}

// ── Test 列数 ────────────────────────────────────────────────────

func Test_colCounts(t *testing.T) {
	t.Logf("permCols count: %d", len(permCols()))
	t.Logf("permRow count (no approve_time): %d", len(newPermRow()))
	assert.Equal(t, len(permCols()), len(newPermRow()), "cols vs row count mismatch")
}

// ── GetByID ─────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPermissionByID)).
		WithArgs(testPermID).
		WillReturnRows(sqlmock.NewRows(permCols()).AddRow(newPermRow()...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testPermID)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, testPermID, p.ID)
	assert.Equal(t, testOrgID, p.OrganizationID)
	assert.Equal(t, testPermName, p.Name)
	assert.Equal(t, testPermDispName, p.DisplayName)
	assert.Equal(t, []string{"user1", "user2"}, p.Users)
	assert.Equal(t, []string{"admin"}, p.Roles)
	assert.Equal(t, true, p.IsEnabled)
	assert.Equal(t, int64(1), p.Revision)
	assert.Nil(t, p.ApproveTime)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPermissionByID)).
		WithArgs("unknown-id").
		WillReturnRows(sqlmock.NewRows(permCols()))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), "unknown-id")
	assert.Error(t, err)
	assert.Nil(t, p)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPermissionByIDForOrganization)).
		WithArgs(testPermID, testOrgID).
		WillReturnRows(sqlmock.NewRows(permCols()).AddRow(newPermRow()...))

	r := NewRepository(db)
	p, err := r.GetByIDForOrganization(context.Background(), testPermID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, testOrgID, p.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_ForeignTenantNotFound(t *testing.T) {
	assert.Contains(t, queryGetPermissionByIDForOrganization, "organization_id = ?")
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPermissionByIDForOrganization)).
		WithArgs(testPermID, "other-org").
		WillReturnRows(sqlmock.NewRows(permCols()))

	r := NewRepository(db)
	p, err := r.GetByIDForOrganization(context.Background(), testPermID, "other-org")
	assert.Error(t, err)
	assert.Nil(t, p)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── List ────────────────────────────────────────────────────────

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// List: org_id × 2, keyword × 4, is_enabled × 2, state × 2, resource_type × 2, effect × 2, limit, offset = 16
	mock.ExpectQuery(regexp.QuoteMeta(queryListPermissions)).
		WithArgs(
			"", "", // org_id × 2
			"", "", "", "", // keyword × 4
			nil, nil, // is_enabled × 2
			"", "", // state × 2
			"", "", // resource_type × 2
			"", "", // effect × 2
			20, 0, // limit, offset
		).
		WillReturnRows(sqlmock.NewRows(permCols()).AddRow(newPermRow()...))

	// Count: 与 List 同条件，去掉 limit/offset = 14
	mock.ExpectQuery(regexp.QuoteMeta(queryCountPermissions)).
		WithArgs(
			"", "",
			"", "", "", "",
			nil, nil,
			"", "",
			"", "",
			"", "",
		).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, testPermID, items[0].ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_WithKeyword(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListPermissions)).
		WithArgs(
			"", "",
			"test", "test", "test", "test",
			nil, nil,
			"", "",
			"", "",
			"", "",
			20, 0,
		).
		WillReturnRows(sqlmock.NewRows(permCols()))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountPermissions)).
		WithArgs(
			"", "",
			"test", "test", "test", "test",
			nil, nil,
			"", "",
			"", "",
			"", "",
		).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20, Keyword: "test"})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// INSERT SQL: 25 个 ?
	mock.ExpectExec(regexp.QuoteMeta(queryInsertPermission)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	p := &domain.Permission{
		ID:             testPermID,
		OrganizationID: testOrgID,
		Name:           testPermName,
		DisplayName:    testPermDispName,
		Description:    testPermDesc,
		Users:          []string{"user1"},
		ResourceType:   testPermResType,
		Resources:      []string{"res1"},
		Actions:        []string{"read"},
		Effect:         testPermEffect,
		IsEnabled:      true,
		State:          "pending",
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       1,
	}
	err := r.Create(context.Background(), p)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ───────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	assert.Contains(t, queryUpdatePermission, "organization_id = ?", "update must carry the entity organization predicate")
	db, mock := newMockDB(t)
	defer db.Close()

	// UPDATE SQL: 20 SET 值 + updated_at + id + old_revision + organization_id = 23 个 ?
	mock.ExpectExec(regexp.QuoteMeta(queryUpdatePermission)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), // display_name, description
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), // users, groups, roles, domains
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), // resource_type, resources, actions
			sqlmock.AnyArg(), sqlmock.AnyArg(), // effect, is_enabled
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), // submitter, approver, state
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), // model, adapter, sorter
			sqlmock.AnyArg(), sqlmock.AnyArg(), // application, approve_time
			sqlmock.AnyArg(),                // updated_at
			testPermID, int64(1), testOrgID, // id, old_revision, organization_id
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// SELECT revision 回读
	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevision)).
		WithArgs(testPermID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	p := &domain.Permission{
		ID:             testPermID,
		OrganizationID: testOrgID,
		DisplayName:    "Updated",
		Description:    "Updated desc",
		Revision:       2,
		UpdatedAt:      fixedNow,
	}
	err := r.Update(context.Background(), p)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), p.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// UPDATE: 0 rows affected = conflict
	mock.ExpectExec(regexp.QuoteMeta(queryUpdatePermission)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			testPermID, int64(98), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	p := &domain.Permission{
		ID:             testPermID,
		OrganizationID: testOrgID,
		Revision:       99,
		UpdatedAt:      fixedNow,
	}
	err := r.Update(context.Background(), p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ───────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	assert.Contains(t, queryDeletePermission, "organization_id = ?", "delete must carry the trusted tenant predicate")
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeletePermission)).
		WithArgs(testPermID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testPermID, testOrgID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_ForeignTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeletePermission)).
		WithArgs(testPermID, "other-org").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testPermID, "other-org")
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
