package deviceauthcodes

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedLater = time.Date(2026, 9, 24, 10, 30, 0, 0, time.UTC)
	testID     = "00000000-0000-0000-0000-000000000010"
	testOrgID  = "00000000-0000-0000-0000-000000000001"
	testUserID = "00000000-0000-0000-0000-000000000002"
	testAppID  = "00000000-0000-0000-0000-000000000003"
	deviceCode = "abcd1234"
	userCode   = "user12345"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// deviceAuthCodeCols: 17 列
func deviceAuthCodeCols() []string {
	return []string{
		"id", "device_code", "user_code", "client_id", "scope", "expires_at", "poll_interval_sec",
		"verified", "verified_at", "user_id", "username", "organization_id",
		"last_polled_at", "slow_down_count", "created_at", "updated_at", "revision",
	}
}

func newDeviceAuthCodeRow() []driver.Value {
	return []driver.Value{
		testID, deviceCode, userCode, testAppID, "openid", fixedLater, 5,
		false, nil, testUserID, "alice", testOrgID,
		nil, 0, fixedNow, fixedNow, int64(1),
	}
}

// ── GetByDeviceCode ───────────────────────────────────────────────

func TestRepository_GetByDeviceCode_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByDeviceCode)).
		WithArgs(deviceCode).
		WillReturnRows(sqlmock.NewRows(deviceAuthCodeCols()).AddRow(newDeviceAuthCodeRow()...))

	r := NewRepository(db)
	c, err := r.GetByDeviceCode(context.Background(), deviceCode)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, deviceCode, c.DeviceCode)
	assert.Equal(t, 5, c.PollIntervalSec)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByDeviceCode_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByDeviceCode)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(deviceAuthCodeCols()))

	r := NewRepository(db)
	_, err := r.GetByDeviceCode(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetByUserCode ────────────────────────────────────────────────

func TestRepository_GetByUserCode_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByUserCode)).
		WithArgs(userCode).
		WillReturnRows(sqlmock.NewRows(deviceAuthCodeCols()).AddRow(newDeviceAuthCodeRow()...))

	r := NewRepository(db)
	c, err := r.GetByUserCode(context.Background(), userCode)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, userCode, c.UserCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryCreate)).
		WithArgs(
			testID, deviceCode, userCode, testAppID, "openid",
			fixedLater, 5, false, nil,
			testUserID, "alice", testOrgID,
			nil, 0, fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.DeviceAuthCode{
		ID:              testID,
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		ClientID:        testAppID,
		Scope:           "openid",
		ExpiresAt:       fixedLater,
		PollIntervalSec: 5,
		Verified:        false,
		UserID:          testUserID,
		Username:        "alice",
		OrganizationID:  testOrgID,
		SlowDownCount:   0,
		CreatedAt:       fixedNow,
		UpdatedAt:       fixedNow,
		Revision:        1,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── RecordPoll ──────────────────────────────────────────────────

func TestRepository_RecordPoll_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// UPDATE 两步参数
	mock.ExpectExec(regexp.QuoteMeta(queryRecordPoll)).
		WithArgs(fixedNow, fixedNow, deviceCode).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// SELECT slow_down_count
	mock.ExpectQuery(regexp.QuoteMeta(querySelectSlowDownAfterPoll)).
		WithArgs(deviceCode).
		WillReturnRows(sqlmock.NewRows([]string{"slow_down_count"}).AddRow(1))

	r := NewRepository(db)
	count, err := r.RecordPoll(context.Background(), deviceCode, fixedNow)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_RecordPoll_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryRecordPoll)).
		WithArgs(fixedNow, fixedNow, "bad").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	_, err := r.RecordPoll(context.Background(), "bad", fixedNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── ResetSlowDown ────────────────────────────────────────────────

func TestRepository_ResetSlowDown(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryResetSlowDown)).
		WithArgs(fixedNow, fixedNow, deviceCode).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.ResetSlowDown(context.Background(), deviceCode, fixedNow)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── UpdateVerification ────────────────────────────────────────────

func TestRepository_UpdateVerification_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateVerification)).
		WithArgs(
			true, fixedNow, testUserID, "alice", testOrgID,
			fixedNow, deviceCode, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	code := &domain.DeviceAuthCode{
		DeviceCode:     deviceCode,
		Verified:       true,
		VerifiedAt:     &fixedNow,
		UserID:         testUserID,
		Username:       "alice",
		OrganizationID: testOrgID,
		UpdatedAt:      fixedNow,
		Revision:       1,
	}
	err := r.UpdateVerification(context.Background(), code)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_UpdateVerification_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateVerification)).
		WithArgs(
			true, fixedNow, testUserID, "alice", testOrgID,
			fixedNow, deviceCode, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	code := &domain.DeviceAuthCode{
		DeviceCode:     deviceCode,
		Verified:       true,
		VerifiedAt:     &fixedNow,
		UserID:         testUserID,
		Username:       "alice",
		OrganizationID: testOrgID,
		UpdatedAt:      fixedNow,
		Revision:       1,
	}
	err := r.UpdateVerification(context.Background(), code)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modified")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_DeleteByDeviceCode(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteByDeviceCode)).
		WithArgs(deviceCode).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.DeleteByDeviceCode(context.Background(), deviceCode)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_DeleteByUserCode(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteByUserCode)).
		WithArgs(userCode).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.DeleteByUserCode(context.Background(), userCode)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── DeleteExpired ─────────────────────────────────────────────────

func TestRepository_DeleteExpired(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteExpired)).
		WillReturnResult(sqlmock.NewResult(3, 3))

	r := NewRepository(db)
	n, err := r.DeleteExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
