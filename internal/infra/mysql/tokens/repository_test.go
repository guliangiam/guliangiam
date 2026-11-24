package tokens

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow      = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testTokenID   = "00000000-0000-0000-0000-000000000001"
	testUserID    = "00000000-0000-0000-0000-000000000002"
	testOrgID     = "00000000-0000-0000-0000-000000000003"
	testAppID     = "00000000-0000-0000-0000-000000000004"
	testRefreshID = "00000000-0000-0000-0000-000000000005"
	testFamilyID  = "family-001"
	testPrefix    = "imp"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── Access Token 列（19 列，匹配 tokenRow Scan 顺序）────────

func atCols() []string {
	return []string{
		"id", "user_id", "username", "organization_id", "organization_slug",
		"application_id", "application_slug", "token_prefix", "token_hash",
		"device_secret_hash", "device_secret_expires_in",
		"scopes", "audience", "metadata",
		"expires_at", "revoked_at", "created_at", "updated_at", "revision",
	}
}

func newATRow() []driver.Value {
	return []driver.Value{
		testTokenID, testUserID, "alice", testOrgID, "my-org",
		testAppID, "my-app", "at_wXYZ", "hash-of-token",
		"device-secret", int64(7200),
		`["openid","profile"]`, `["my-app"]`, `{}`,
		fixedNow.Add(1 * time.Hour), nil, fixedNow, fixedNow, int64(1),
	}
}

// ── Refresh Token 列（17 列，匹配 refreshTokenRow Scan 顺序）──

func rtCols() []string {
	return []string{
		"id", "access_token_id", "user_id", "username",
		"organization_id", "organization_slug",
		"application_id", "application_slug",
		"token_hash", "family_id",
		"scopes", "audience",
		"expires_at", "revoked_at", "created_at", "updated_at", "revision",
	}
}

func newRTRow() []driver.Value {
	return []driver.Value{
		testRefreshID, testTokenID, testUserID, "alice",
		testOrgID, "my-org",
		testAppID, "my-app",
		"hash-of-refresh", testFamilyID,
		`["openid","profile"]`, `["my-app"]`,
		fixedNow.Add(24 * time.Hour), nil, fixedNow, fixedNow, int64(1),
	}
}

// ── Access Token SELECTs ────────────────────────────────────────

func TestRepository_GetByID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetTokenByID)).
		WithArgs(testTokenID).
		WillReturnRows(sqlmock.NewRows(atCols()).AddRow(newATRow()...))

	r := NewRepository(db)
	t2, err := r.GetByID(context.Background(), testTokenID)
	assert.NoError(t, err)
	assert.NotNil(t, t2)
	assert.Equal(t, testTokenID, t2.ID)
	assert.Equal(t, "alice", t2.Username)
	assert.Equal(t, []string{"openid", "profile"}, t2.Scopes)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetTokenByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(atCols()))

	r := NewRepository(db)
	t2, err := r.GetByID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, t2)
}

func TestRepository_GetByTokenHash(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetTokenByHash)).
		WithArgs("hash-of-token").
		WillReturnRows(sqlmock.NewRows(atCols()).AddRow(newATRow()...))

	r := NewRepository(db)
	t2, err := r.GetByTokenHash(context.Background(), "hash-of-token")
	assert.NoError(t, err)
	assert.Equal(t, testTokenID, t2.ID)
}

func TestRepository_GetByDeviceSecretHash(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetTokenByDeviceSecretHash)).
		WithArgs("device-secret").
		WillReturnRows(sqlmock.NewRows(atCols()).AddRow(newATRow()...))

	r := NewRepository(db)
	t2, err := r.GetByDeviceSecretHash(context.Background(), "device-secret")
	assert.NoError(t, err)
	assert.Equal(t, testTokenID, t2.ID)
}

// ── Refresh Token SELECT ────────────────────────────────────────

func TestRepository_GetRefreshByTokenHash(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetRefreshTokenByHash)).
		WithArgs("hash-of-refresh").
		WillReturnRows(sqlmock.NewRows(rtCols()).AddRow(newRTRow()...))

	r := NewRepository(db)
	rt, err := r.GetRefreshByTokenHash(context.Background(), "hash-of-refresh")
	assert.NoError(t, err)
	assert.NotNil(t, rt)
	assert.Equal(t, testRefreshID, rt.ID)
	assert.Equal(t, testFamilyID, rt.FamilyID)
}

// ── List ────────────────────────────────────────────────────────
// List 占位符 17 个：org×2 + user×2 + app×2 + active×3 + keyword×6 + limit + offset
// Count 占位符 15 个：同上去掉 limit/offset

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// nil activeOnly → active_only × 3 = nil
	listArgs := []driver.Value{
		"", "", // org×2
		"", "", // user×2
		"", "", // app×2
		nil, nil, nil, // active×3
		"", "", "", "", "", "", // keyword×6
		20, 0, // limit, offset
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryListTokens)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(atCols()).AddRow(newATRow()...))

	countArgs := []driver.Value{
		"", "", "", "", "", "",
		nil, nil, nil,
		"", "", "", "", "", "",
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountTokens)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
}

func TestRepository_List_WithFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	active := true
	listArgs := []driver.Value{
		testOrgID, testOrgID,
		testUserID, testUserID,
		testAppID, testAppID,
		active, active, active,
		"alice", "alice", "alice", "alice", "alice", "alice",
		20, 0,
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryListTokens)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(atCols()).AddRow(newATRow()...))

	countArgs := []driver.Value{
		testOrgID, testOrgID, testUserID, testUserID, testAppID, testAppID,
		active, active, active,
		"alice", "alice", "alice", "alice", "alice", "alice",
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountTokens)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID, UserID: testUserID, ApplicationID: testAppID,
		ActiveOnly: &active, Keyword: "alice", Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	scopesJSON, _ := json.Marshal([]string{"openid", "profile"})
	audienceJSON, _ := json.Marshal([]string{"my-app"})

	mock.ExpectExec(regexp.QuoteMeta(queryInsertToken)).
		WithArgs(
			testTokenID, testUserID, testAppID, "hash-of-token", "at_wXYZ",
			nil, 7200,
			string(scopesJSON), string(audienceJSON),
			`{"key":"value"}`,
			fixedNow.Add(1*time.Hour), nil, fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Token{
		ID:                    testTokenID,
		UserID:                testUserID,
		ApplicationID:         testAppID,
		TokenHash:             "hash-of-token",
		TokenPrefix:           "at_wXYZ",
		DeviceSecretExpiresIn: 7200,
		Scopes:                []string{"openid", "profile"},
		Audience:              []string{"my-app"},
		Metadata:              map[string]any{"key": "value"},
		ExpiresAt:             fixedNow.Add(1 * time.Hour),
		CreatedAt:             fixedNow,
		UpdatedAt:             fixedNow,
		Revision:              int64(1),
	})
	assert.NoError(t, err)
}

func TestRepository_UpdateDeviceSecret(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateTokenDeviceSecret)).
		WithArgs("secret-hash", 7200, testTokenID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	r := NewRepository(db)
	err := r.UpdateDeviceSecret(context.Background(), testTokenID, "secret-hash", 7200)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_UpdateDeviceSecret_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateTokenDeviceSecret)).
		WithArgs("secret-hash", 7200, testTokenID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.UpdateDeviceSecret(context.Background(), testTokenID, "secret-hash", 7200)
	assert.Error(t, err)
}

func TestRepository_CreateRefresh(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertRefreshToken)).
		WithArgs(
			testRefreshID, testTokenID, testUserID, testAppID,
			"hash-of-refresh", "", testFamilyID,
			`["openid","profile"]`, `["my-app"]`,
			fixedNow.Add(24*time.Hour), nil, fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.CreateRefresh(context.Background(), &domain.RefreshToken{
		ID:               testRefreshID,
		AccessTokenID:    testTokenID,
		UserID:           testUserID,
		ApplicationID:    testAppID,
		RefreshTokenHash: "hash-of-refresh",
		FamilyID:         testFamilyID,
		Scopes:           []string{"openid", "profile"},
		Audience:         []string{"my-app"},
		ExpiresAt:        fixedNow.Add(24 * time.Hour),
		CreatedAt:        fixedNow,
		UpdatedAt:        fixedNow,
		Revision:         int64(1),
	})
	assert.NoError(t, err)
}

// ── Revoke ────────────────────────────────────────────────────────

func TestRepository_Revoke_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	revokedAt := fixedNow.Add(time.Minute)

	// queryRevokeToken 占位符：revoked_at, updated_at, id = 3
	mock.ExpectExec(regexp.QuoteMeta(queryRevokeToken)).
		WithArgs(revokedAt, revokedAt, testTokenID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Revoke(context.Background(), testTokenID, revokedAt)
	assert.NoError(t, err)
}

func TestRepository_RevokeRefresh_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	revokedAt := fixedNow.Add(time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeRefreshToken)).
		WithArgs(revokedAt, revokedAt, testRefreshID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.RevokeRefresh(context.Background(), testRefreshID, revokedAt)
	assert.NoError(t, err)
}

func TestRepository_RevokeRefresh_AlreadyRevoked(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	revokedAt := fixedNow.Add(time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeRefreshToken)).
		WithArgs(revokedAt, revokedAt, testRefreshID).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.RevokeRefresh(context.Background(), testRefreshID, revokedAt)
	assert.ErrorIs(t, err, domain.ErrRefreshAlreadyRevoked)
}

// ── RevokeByQuery ────────────────────────────────────────────────
// 占位符：revoked_at(2), org_id×2, user_id×2, app_id×2, token_prefix×2, expires_at(1) = 11

func TestRepository_RevokeByQuery(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	revokedAt := fixedNow.Add(time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeTokensByQuery)).
		WithArgs(
			revokedAt, revokedAt,
			testOrgID, testOrgID,
			testUserID, testUserID,
			testAppID, testAppID,
			testPrefix, testPrefix,
			revokedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 3))

	r := NewRepository(db)
	count, err := r.RevokeByQuery(context.Background(), domain.RevokeQuery{
		OrganizationID: testOrgID, UserID: testUserID, ApplicationID: testAppID,
		TokenPrefix: testPrefix,
	}, revokedAt)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// ── RevokeByFamilyID ─────────────────────────────────────────────
// 两步 UPDATE：
//   1. queryRevokeRefreshByFamily: revoked_at, revoked_at, family_id = 3
//   2. queryRevokeAccessByFamilyJoin: family_id, revoked_at, revoked_at = 3

func TestRepository_RevokeByFamilyID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	revokedAt := fixedNow.Add(time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeRefreshByFamily)).
		WithArgs(revokedAt, revokedAt, testFamilyID).
		WillReturnResult(sqlmock.NewResult(1, 2))

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeAccessByFamilyJoin)).
		WithArgs(testFamilyID, revokedAt, revokedAt).
		WillReturnResult(sqlmock.NewResult(1, 2))

	r := NewRepository(db)
	count, err := r.RevokeByFamilyID(context.Background(), testFamilyID, revokedAt)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// ── RevokeRefreshByUser ──────────────────────────────────────────
// 两步 UPDATE，参数结构同 RevokeByFamilyID

func TestRepository_RevokeRefreshByUser(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	revokedAt := fixedNow.Add(time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeRefreshByUser)).
		WithArgs(revokedAt, revokedAt, testUserID).
		WillReturnResult(sqlmock.NewResult(1, 3))

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeAccessByUserJoin)).
		WithArgs(testUserID, revokedAt, revokedAt).
		WillReturnResult(sqlmock.NewResult(1, 3))

	r := NewRepository(db)
	count, err := r.RevokeRefreshByUser(context.Background(), testUserID, revokedAt)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// ── RevokeByAuthorizationCodeID ────────────────────────────────────
// 两步 UPDATE：
//   1. queryRevokeAccessByCodeID: revoked_at, revoked_at, codeID = 3
//   2. queryRevokeRefreshByAccessToken: revoked_at, revoked_at, codeID, revokedAt = 4

func TestRepository_RevokeByAuthorizationCodeID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	revokedAt := fixedNow.Add(time.Minute)
	codeID := "auth-code-001"

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeAccessByCodeID)).
		WithArgs(revokedAt, revokedAt, codeID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeRefreshByAccessToken)).
		WithArgs(revokedAt, revokedAt, codeID, revokedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	count, err := r.RevokeByAuthorizationCodeID(context.Background(), codeID, revokedAt)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteToken)).
		WithArgs(testTokenID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testTokenID)
	assert.NoError(t, err)
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteToken)).
		WithArgs("bad").
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), "bad")
	assert.Error(t, err)
}

// 编译期接口断言
var _ domain.Repository = Repository{}

// TestRepository_GetByIDForOrganization_TenantPredicate pins that the MySQL by-ID
// read carries the explicit organization_id predicate through the users join.
// MySQL has no RLS, so this predicate is the only tenant boundary.
func TestRepository_GetByIDForOrganization_TenantPredicate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetTokenByIDForOrganization)).
		WithArgs(testTokenID, testOrgID).
		WillReturnRows(sqlmock.NewRows(atCols()))

	_, err := NewRepository(db).GetByIDForOrganization(context.Background(), testTokenID, testOrgID)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
