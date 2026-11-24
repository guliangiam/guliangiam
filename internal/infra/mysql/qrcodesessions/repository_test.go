package qrcodesessions

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow7  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedLater = time.Date(2026, 9, 24, 10, 15, 0, 0, time.UTC)
	testOrgID7 = "00000000-0000-0000-0000-000000000001"
	testAppID7 = "00000000-0000-0000-0000-000000000002"
	testQRID   = "00000000-0000-0000-0000-000000000005"
	testCodeID = "qr-code-001" // naming-allow: legacy test fixture"
)

func newQRMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// qrCols: 14 列
func qrCols() []string {
	return []string{
		"id", "code_id", "code", "organization_id", "application_id", "expires_at", "status",
		"user_id", "username", "ip_address", "user_agent", "created_at", "updated_at", "revision",
	}
}

func newQRRow() []driver.Value {
	return []driver.Value{
		testQRID, testCodeID, "https://qr.example.com/abc", testOrgID7, testAppID7,
		fixedLater, "pending", "", "", "", "", fixedNow7, fixedNow7, int64(1),
	}
}

// ── GetByCodeID ───────────────────────────────────────────────────────

func TestRepository_GetByCodeID_Found(t *testing.T) {
	db, mock := newQRMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByCodeID)).
		WithArgs(testCodeID).
		WillReturnRows(sqlmock.NewRows(qrCols()).AddRow(newQRRow()...))

	r := NewRepository(db)
	s, err := r.GetByCodeID(context.Background(), testCodeID)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, testQRID, s.ID)
	assert.Equal(t, "pending", s.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByCodeID_NotFound(t *testing.T) {
	db, mock := newQRMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByCodeID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(qrCols()))

	r := NewRepository(db)
	s, err := r.GetByCodeID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, s)
}

// ── GetByCode ─────────────────────────────────────────────────────

func TestRepository_GetByCode_Found(t *testing.T) {
	db, mock := newQRMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByCode)).
		WithArgs("qr-payload-001").
		WillReturnRows(sqlmock.NewRows(qrCols()).AddRow(newQRRow()...))

	r := NewRepository(db)
	s, err := r.GetByCode(context.Background(), "qr-payload-001")
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, testQRID, s.ID)
	assert.Equal(t, testCodeID, s.CodeID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newQRMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryCreate)).
		WithArgs(
			testQRID, testCodeID, "https://qr.example.com/abc", testOrgID7, testAppID7,
			fixedLater, "pending", "", "", "", "", fixedNow7, fixedNow7, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Session{
		ID:             testQRID,
		CodeID:         testCodeID,
		Code:           "https://qr.example.com/abc",
		OrganizationID: testOrgID7,
		ApplicationID:  testAppID7,
		ExpiresAt:      fixedLater,
		Status:         "pending",
		CreatedAt:      fixedNow7,
		UpdatedAt:      fixedNow7,
		Revision:       int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── UpdateStatus ──────────────────────────────────────────────────

func TestRepository_UpdateStatus_Success(t *testing.T) {
	db, mock := newQRMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateStatus)).
		WithArgs(
			"confirmed", "user-001", "alice",
			"192.168.1.1", "Mozilla/5.0", fixedNow7,
			testCodeID, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.UpdateStatus(context.Background(), &domain.Session{
		CodeID:    testCodeID,
		Revision:  int64(1),
		Status:    "confirmed",
		UserID:    "user-001",
		Username:  "alice",
		IPAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
		UpdatedAt: fixedNow7,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_UpdateStatus_Conflict(t *testing.T) {
	db, mock := newQRMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateStatus)).
		WithArgs(
			"confirmed", "user-001", "alice",
			"192.168.1.1", "Mozilla/5.0", fixedNow7,
			testCodeID, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.UpdateStatus(context.Background(), &domain.Session{
		CodeID:    testCodeID,
		Revision:  int64(1),
		Status:    "confirmed",
		UserID:    "user-001",
		Username:  "alice",
		IPAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
		UpdatedAt: fixedNow7,
	})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── DeleteByCodeID ────────────────────────────────────────────────

func TestRepository_DeleteByCodeID(t *testing.T) {
	db, mock := newQRMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteByCodeID)).
		WithArgs(testCodeID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.DeleteByCodeID(context.Background(), testCodeID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── DeleteExpired ─────────────────────────────────────────────────

func TestRepository_DeleteExpired(t *testing.T) {
	db, mock := newQRMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteExpired)).
		WillReturnResult(sqlmock.NewResult(0, 5))

	r := NewRepository(db)
	n, err := r.DeleteExpired(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(5), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
