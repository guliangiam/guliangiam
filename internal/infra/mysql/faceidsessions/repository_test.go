package faceidsessions

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow      = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedExpiry   = time.Date(2026, 9, 24, 15, 0, 0, 0, time.UTC)
	testSessionID = "face-session-abc"
	testUserID    = "00000000-0000-0000-0000-000000000001"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// sessionCols: 10 列（含 attempt_count）
func sessionCols() []string {
	return []string{
		"id", "session_id", "challenge", "user_id", "expires_at", "verified", "liveness_ok", "attempt_count", "created_at", "updated_at",
	}
}

func newSessionRow() []driver.Value {
	return []driver.Value{
		"00000000-0000-0000-0000-000000000010", testSessionID, "random-challenge", testUserID,
		fixedExpiry, false, false, 0, fixedNow, fixedNow,
	}
}

// ── GetBySessionID ───────────────────────────────────────────────

func TestRepository_GetBySessionID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetBySessionID)).
		WithArgs(testSessionID).
		WillReturnRows(sqlmock.NewRows(sessionCols()).AddRow(newSessionRow()...))

	r := NewRepository(db)
	s, err := r.GetBySessionID(context.Background(), testSessionID)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, testSessionID, s.SessionID)
	assert.Equal(t, false, s.Verified)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetBySessionID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetBySessionID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(sessionCols()))

	r := NewRepository(db)
	s, err := r.GetBySessionID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, s)
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryCreate)).
		WithArgs(
			"00000000-0000-0000-0000-000000000010", testSessionID, "random-challenge", testUserID,
			fixedExpiry, false, false, fixedNow, fixedNow,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Session{
		ID:        "00000000-0000-0000-0000-000000000010",
		SessionID: testSessionID,
		Challenge: "random-challenge",
		UserID:    testUserID,
		ExpiresAt: fixedExpiry,
		CreatedAt: fixedNow,
		UpdatedAt: fixedNow,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ───────────────────────────────────────────────────────

func TestRepository_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdate)).
		WithArgs(true, true, 0, fixedNow, testSessionID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Session{
		SessionID:    testSessionID,
		Verified:     true,
		LivenessOK:   true,
		AttemptCount: 0,
		UpdatedAt:    fixedNow,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdate)).
		WithArgs(true, true, 0, fixedNow, testSessionID).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 affected → conflict

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Session{
		SessionID:    testSessionID,
		Verified:     true,
		LivenessOK:   true,
		AttemptCount: 0,
		UpdatedAt:    fixedNow,
	})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── ConsumeVerified ──────────────────────────────────────────────

func TestRepository_ConsumeVerified_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryConsumeVerified)).
		WithArgs(testSessionID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.ConsumeVerified(context.Background(), testSessionID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ConsumeVerified_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryConsumeVerified)).
		WithArgs(testSessionID).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 affected → conflict

	r := NewRepository(db)
	err := r.ConsumeVerified(context.Background(), testSessionID)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── DeleteBySessionID ────────────────────────────────────────────

func TestRepository_DeleteBySessionID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteBySessionID)).
		WithArgs(testSessionID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.DeleteBySessionID(context.Background(), testSessionID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── DeleteExpired ──────────────────────────────────────────────────

func TestRepository_DeleteExpired(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteExpired)).
		WillReturnResult(sqlmock.NewResult(5, 5))

	r := NewRepository(db)
	n, err := r.DeleteExpired(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(5), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
