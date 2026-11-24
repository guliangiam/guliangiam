package lockoutattempts

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedLater = time.Date(2026, 9, 24, 10, 30, 0, 0, time.UTC)
	testUserID = "00000000-0000-0000-0000-000000000001"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

func attemptCols() []string {
	return []string{
		"id", "user_id", "attempts", "locked_until", "window_started_at", "lockout_level", "created_at", "updated_at", "revision",
	}
}

func newAttemptRow() []driver.Value {
	return []driver.Value{
		"00000000-0000-0000-0000-000000000010", testUserID, 3, nil, fixedNow, 0,
		fixedNow, fixedNow, int64(1),
	}
}

func newLockedAttemptRow() []driver.Value {
	return []driver.Value{
		"00000000-0000-0000-0000-000000000010", testUserID, 5, fixedLater, fixedNow, 1,
		fixedNow, fixedNow, int64(1),
	}
}

// ── GetByUserID ───────────────────────────────────────────────────

func TestRepository_GetByUserID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByUserID)).
		WithArgs(testUserID).
		WillReturnRows(sqlmock.NewRows(attemptCols()).AddRow(newAttemptRow()...))

	r := NewRepository(db)
	a, err := r.GetByUserID(context.Background(), testUserID)
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, testUserID, a.UserID)
	assert.Equal(t, 3, a.Attempts)
	assert.Equal(t, int64(1), a.Revision)
	assert.Nil(t, a.LockedUntil)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByUserID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByUserID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(attemptCols()))

	r := NewRepository(db)
	a, err := r.GetByUserID(context.Background(), "bad")
	assert.NoError(t, err) // 没找到不是 error
	assert.Nil(t, a)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByUserID_Locked(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByUserID)).
		WithArgs(testUserID).
		WillReturnRows(sqlmock.NewRows(attemptCols()).AddRow(newLockedAttemptRow()...))

	r := NewRepository(db)
	a, err := r.GetByUserID(context.Background(), testUserID)
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, 5, a.Attempts)
	assert.NotNil(t, a.LockedUntil)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryCreate)).
		WithArgs(
			"00000000-0000-0000-0000-000000000010", testUserID, 3, nil,
			fixedNow, 0, fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Attempt{
		ID:              "00000000-0000-0000-0000-000000000010",
		UserID:          testUserID,
		Attempts:        3,
		WindowStartedAt: fixedNow,
		Revision:        1,
		CreatedAt:       fixedNow,
		UpdatedAt:       fixedNow,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdate)).
		WithArgs(4, nil, fixedNow, 0, fixedNow, testUserID, int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	attempt := &domain.Attempt{
		UserID:          testUserID,
		Attempts:        4,
		WindowStartedAt: fixedNow,
		Revision:        1,
		UpdatedAt:       fixedNow,
	}
	err := r.Update(context.Background(), attempt)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), attempt.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdate)).
		WithArgs(4, nil, fixedNow, 0, fixedNow, testUserID, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	attempt := &domain.Attempt{
		UserID:          testUserID,
		Attempts:        4,
		WindowStartedAt: fixedNow,
		Revision:        1,
		UpdatedAt:       fixedNow,
	}
	err := r.Update(context.Background(), attempt)
	assert.Error(t, err)
	assert.Equal(t, apperrors.CodeConflict, err.(*apperrors.Error).Code)
	assert.Equal(t, int64(1), attempt.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── RecordFailure ────────────────────────────────────────────────

func TestRepository_RecordFailure(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(queryEnsureAttempt)).
		WithArgs(sqlmock.AnyArg(), testUserID, fixedNow, fixedNow, fixedNow).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(queryLockByUserID)).
		WithArgs(testUserID).
		WillReturnRows(sqlmock.NewRows(attemptCols()).AddRow(newAttemptRow()...))

	mock.ExpectExec(regexp.QuoteMeta(queryUpdate)).
		WithArgs(4, nil, fixedNow, 0, fixedNow, testUserID, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	r := NewRepository(db)
	a, err := r.RecordFailure(context.Background(), testUserID, 5, 30, fixedNow)
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, testUserID, a.UserID)
	assert.Equal(t, 4, a.Attempts)
}

// ── DeleteByUserID ────────────────────────────────────────────────

func TestRepository_DeleteByUserID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteByUserID)).
		WithArgs(testUserID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.DeleteByUserID(context.Background(), testUserID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── DeleteExpired ──────────────────────────────────────────────────

func TestRepository_DeleteExpired(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteExpired)).
		WillReturnResult(sqlmock.NewResult(2, 2))

	r := NewRepository(db)
	n, err := r.DeleteExpired(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(2), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
