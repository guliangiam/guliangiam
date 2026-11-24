package invitations

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedLater = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	testID     = "00000000-0000-0000-0000-000000000010"
	testOrgID  = "00000000-0000-0000-0000-000000000001"
	testUserID = "00000000-0000-0000-0000-000000000002"
	testEmail  = "test@example.com"
	testCode   = "INV12345"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// invitationCols: 13 列
func invitationCols() []string {
	return []string{
		"id", "organization_id", "email", "code", "role_ids", "group_ids", "status",
		"expires_at", "accepted_at", "invited_by", "created_at", "updated_at", "revision",
	}
}

func newInvitationRow() []driver.Value {
	return []driver.Value{
		testID, testOrgID, testEmail, testCode,
		`["role-1","role-2"]`, `["group-1"]`, "pending",
		fixedLater, nil, testUserID,
		fixedNow, fixedNow, int64(1),
	}
}

// ── GetByID ─────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetInvitationByID)).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows(invitationCols()).AddRow(newInvitationRow()...))

	r := NewRepository(db)
	inv, err := r.GetByID(context.Background(), testID)
	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.Equal(t, testID, inv.ID)
	assert.Equal(t, testEmail, inv.Email)
	assert.Equal(t, []string{"role-1", "role-2"}, inv.RoleIDs)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetInvitationByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(invitationCols()))

	r := NewRepository(db)
	_, err := r.GetByID(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetInvitationByIDForOrganization)).
		WithArgs(testID, testOrgID).
		WillReturnRows(sqlmock.NewRows(invitationCols()).AddRow(newInvitationRow()...))

	r := NewRepository(db)
	inv, err := r.GetByIDForOrganization(context.Background(), testID, testOrgID)
	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.Equal(t, testID, inv.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetInvitationByIDForOrganization)).
		WithArgs(testID, "other-org").
		WillReturnRows(sqlmock.NewRows(invitationCols()))

	r := NewRepository(db)
	_, err := r.GetByIDForOrganization(context.Background(), testID, "other-org")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetByCode ────────────────────────────────────────────────────

func TestRepository_GetByCode_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetInvitationByCode)).
		WithArgs(testCode).
		WillReturnRows(sqlmock.NewRows(invitationCols()).AddRow(newInvitationRow()...))

	r := NewRepository(db)
	inv, err := r.GetByCode(context.Background(), testCode)
	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.Equal(t, testCode, inv.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetByEmail ───────────────────────────────────────────────────

func TestRepository_GetByEmail_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetInvitationByEmail)).
		WithArgs(testOrgID, testEmail).
		WillReturnRows(sqlmock.NewRows(invitationCols()).AddRow(newInvitationRow()...))

	r := NewRepository(db)
	inv, err := r.GetByEmail(context.Background(), testOrgID, testEmail)
	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.Equal(t, testEmail, inv.Email)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── List ─────────────────────────────────────────────────────────

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	q := domain.ListQuery{
		Page:           1,
		PageSize:       20,
		OrganizationID: testOrgID,
		Keyword:        "",
		Status:         "",
	}

	limit := 20
	offset := 0

	// List 查询
	mock.ExpectQuery(regexp.QuoteMeta(queryListInvitations)).
		WithArgs(
			testOrgID, testOrgID, // org_id × 2 (CASE WHEN)
			"", "", // keyword × 2 (LIKE CONCAT)
			"", "", // status × 2
			limit, offset,
		).
		WillReturnRows(sqlmock.NewRows(invitationCols()).AddRow(newInvitationRow()...))

	// Count 查询
	mock.ExpectQuery(regexp.QuoteMeta(queryCountInvitations)).
		WithArgs(
			testOrgID, testOrgID,
			"", "",
			"", "",
		).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 13 个参数
	mock.ExpectExec(regexp.QuoteMeta(queryInsertInvitation)).
		WithArgs(
			testID, testOrgID, testEmail, testCode,
			`["role-1"]`, `["group-1"]`, "pending",
			fixedLater, nil, testUserID,
			fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Invitation{
		ID:             testID,
		OrganizationID: testOrgID,
		Email:          testEmail,
		Code:           testCode,
		RoleIDs:        []string{"role-1"},
		GroupIDs:       []string{"group-1"},
		Status:         "pending",
		ExpiresAt:      fixedLater,
		InvitedBy:      testUserID,
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       1,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ───────────────────────────────────────────────────────

func TestRepository_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 12 个参数: email, code, role_ids, group_ids, status, expires_at, accepted_at,
	//           invited_by, updated_at, id, revision, organization_id
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateInvitation)).
		WithArgs(
			testEmail, testCode,
			`["role-1"]`, `["group-1"]`,
			"accepted", fixedLater, nil,
			testUserID, fixedNow,
			testID, int64(1),
			testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevisionAfterUpdate)).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	inv := &domain.Invitation{
		ID:             testID,
		OrganizationID: testOrgID,
		Email:          testEmail,
		Code:           testCode,
		RoleIDs:        []string{"role-1"},
		GroupIDs:       []string{"group-1"},
		Status:         "accepted",
		ExpiresAt:      fixedLater,
		InvitedBy:      testUserID,
		UpdatedAt:      fixedNow,
		Revision:       1,
	}
	err := r.Update(context.Background(), inv)
	require.NoError(t, err)
	assert.Equal(t, int64(2), inv.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateInvitation)).
		WithArgs(
			testEmail, testCode,
			`["role-1"]`, `["group-1"]`,
			"accepted", fixedLater, nil,
			testUserID, fixedNow,
			testID, int64(1),
			testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	inv := &domain.Invitation{
		ID:             testID,
		OrganizationID: testOrgID,
		Email:          testEmail,
		Code:           testCode,
		RoleIDs:        []string{"role-1"},
		GroupIDs:       []string{"group-1"},
		Status:         "accepted",
		ExpiresAt:      fixedLater,
		InvitedBy:      testUserID,
		UpdatedAt:      fixedNow,
		Revision:       1,
	}
	err := r.Update(context.Background(), inv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteInvitation)).
		WithArgs(testID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testID)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteInvitation)).
		WithArgs("bad").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
