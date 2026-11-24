package mfachallenges

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedLater = time.Date(2026, 9, 24, 10, 10, 0, 0, time.UTC)
	testID     = "00000000-0000-0000-0000-000000000010"
	testOrgID  = "00000000-0000-0000-0000-000000000001"
	testUserID = "00000000-0000-0000-0000-000000000002"
	testAppID  = "00000000-0000-0000-0000-000000000003"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// challengeCols: 12 列
func challengeCols() []string {
	return []string{
		"id", "organization_id", "user_id", "application_id", "state", "method_type",
		"metadata", "expires_at", "verified_at", "created_at", "updated_at", "revision",
	}
}

func newChallengeRow() []driver.Value {
	return []driver.Value{
		testID, testOrgID, testUserID, testAppID, "pending", "totp", []byte("{}"),
		fixedLater, nil, fixedNow, fixedNow, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetChallengeByID)).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows(challengeCols()).AddRow(newChallengeRow()...))

	r := NewRepository(db)
	c, err := r.GetByID(context.Background(), testID)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, testID, c.ID)
	assert.Equal(t, "pending", c.State)
	assert.Equal(t, int64(1), c.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetChallengeByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(challengeCols()))

	r := NewRepository(db)
	_, err := r.GetByID(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertChallenge)).
		WithArgs(
			testID, testOrgID, testUserID, testAppID, "pending", "totp", []byte("{}"),
			fixedLater, nil, fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Challenge{
		ID:             testID,
		OrganizationID: testOrgID,
		UserID:         testUserID,
		ApplicationID:  testAppID,
		State:          "pending",
		MethodType:     "totp",
		ExpiresAt:      fixedLater,
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

	// UPDATE: state, method_type, metadata, expires_at, verified_at, updated_at, id, revision
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateChallenge)).
		WithArgs(
			"verified", "totp", []byte("{}"), fixedLater, fixedNow, fixedNow,
			testID, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// SELECT new revision
	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevisionAfterUpdate)).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	c := &domain.Challenge{
		ID:         testID,
		State:      "verified",
		MethodType: "totp",
		ExpiresAt:  fixedLater,
		VerifiedAt: &fixedNow,
		UpdatedAt:  fixedNow,
		Revision:   1,
	}
	err := r.Update(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, int64(2), c.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateChallenge)).
		WithArgs(
			"verified", "totp", []byte("{}"), fixedLater, fixedNow, fixedNow,
			testID, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	c := &domain.Challenge{
		ID:         testID,
		State:      "verified",
		MethodType: "totp",
		ExpiresAt:  fixedLater,
		VerifiedAt: &fixedNow,
		UpdatedAt:  fixedNow,
		Revision:   1,
	}
	err := r.Update(context.Background(), c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteChallenge)).
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

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteChallenge)).
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
