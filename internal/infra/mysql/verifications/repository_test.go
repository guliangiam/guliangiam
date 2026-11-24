package verifications

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow    = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedExpiry = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	testOrgID   = "00000000-0000-0000-0000-000000000001"
	testVerID   = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// verCols: 8 列
func verCols() []string {
	return []string{
		"id", "organization_id", "type", "recipient", "code", "status", "expires_at", "created_at",
	}
}

func newVerRow() []driver.Value {
	return []driver.Value{
		testVerID, testOrgID, "email", "user@example.com", "123456", "pending",
		fixedExpiry, fixedNow,
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetVerificationByID)).
		WithArgs(testVerID).
		WillReturnRows(sqlmock.NewRows(verCols()).AddRow(newVerRow()...))

	r := NewRepository(db)
	v, err := r.GetByID(context.Background(), testVerID)
	assert.NoError(t, err)
	assert.NotNil(t, v)
	assert.Equal(t, testVerID, v.ID)
	assert.Equal(t, "pending", v.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetVerificationByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(verCols()))

	r := NewRepository(db)
	v, err := r.GetByID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestRepository_GetByIDForOrganization_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetVerificationByIDForOrganization)).
		WithArgs(testVerID, testOrgID).
		WillReturnRows(sqlmock.NewRows(verCols()).AddRow(newVerRow()...))

	r := NewRepository(db)
	v, err := r.GetByIDForOrganization(context.Background(), testVerID, testOrgID)
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, testVerID, v.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetVerificationByIDForOrganization)).
		WithArgs(testVerID, "other-org").
		WillReturnRows(sqlmock.NewRows(verCols()))

	r := NewRepository(db)
	v, err := r.GetByIDForOrganization(context.Background(), testVerID, "other-org")
	assert.Error(t, err)
	assert.Nil(t, v)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org×2 + type×2 + status×2 + recipient×3 = 9
// listArgs: countArgs + 20 + 0 = 11
func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{
		testOrgID, testOrgID,
		"email", "email",
		"pending", "pending",
		"user@example.com", "user@example.com", "user@example.com",
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountVerifications)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListVerifications)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(verCols()).AddRow(newVerRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID,
		Type:           "email",
		Status:         "pending",
		Recipient:      "user@example.com",
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
	mock.ExpectQuery(regexp.QuoteMeta(queryCountVerifications)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListVerifications)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(verCols()))

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

	mock.ExpectExec(regexp.QuoteMeta(queryCreateVerification)).
		WithArgs(
			testVerID, testOrgID, "email", "user@example.com", "123456", "pending",
			fixedExpiry, fixedNow,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Verification{
		ID:             testVerID,
		OrganizationID: testOrgID,
		Type:           "email",
		Recipient:      "user@example.com",
		Code:           "123456",
		Status:         "pending",
		ExpiresAt:      fixedExpiry,
		CreatedAt:      fixedNow,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── UpdateStatus ──────────────────────────────────────────────────

func TestRepository_UpdateStatus_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateVerificationStatus)).
		WithArgs("verified", testVerID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.UpdateStatus(context.Background(), testVerID, "verified")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_UpdateStatus_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateVerificationStatus)).
		WithArgs("verified", testVerID).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 affected → conflict

	r := NewRepository(db)
	err := r.UpdateStatus(context.Background(), testVerID, "verified")
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetActiveByRecipientAndCode ───────────────────────────────────

func TestRepository_GetActiveByRecipientAndCode_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetActiveVerificationByRecipientAndCode)).
		WithArgs("user@example.com", "123456", "email").
		WillReturnRows(sqlmock.NewRows(verCols()).AddRow(newVerRow()...))

	r := NewRepository(db)
	v, err := r.GetActiveByRecipientAndCode(context.Background(), "user@example.com", "123456", "email")
	assert.NoError(t, err)
	assert.NotNil(t, v)
	assert.Equal(t, "pending", v.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetActiveByRecipientAndCode_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetActiveVerificationByRecipientAndCode)).
		WillReturnRows(sqlmock.NewRows(verCols()))

	r := NewRepository(db)
	v, err := r.GetActiveByRecipientAndCode(context.Background(), "bad", "bad", "email")
	assert.Error(t, err)
	assert.Nil(t, v)
}

// 编译期接口断言
var _ domain.Repository = Repository{}
