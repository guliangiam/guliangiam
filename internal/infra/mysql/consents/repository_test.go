package consents

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/consents"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow      = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testConsentID = "00000000-0000-0000-0000-000000000001"
	testUserID    = "00000000-0000-0000-0000-000000000002"
	testOrgID     = "00000000-0000-0000-0000-000000000003"
	testAppID     = "00000000-0000-0000-0000-000000000004"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义（12 列，严格匹配 GetByUserAndApplication SELECT 顺序）────

func consentCols() []string {
	return []string{
		"id", "user_id", "username", "organization_id", "organization_slug",
		"application_id", "application_slug", "granted_scopes", "created_at",
		"updated_at", "revoked_at", "revision",
	}
}

func newConsentRow() []driver.Value {
	return []driver.Value{
		testConsentID, testUserID, "alice", testOrgID, "my-org",
		testAppID, "my-app",
		`["openid","profile"]`,
		fixedNow, fixedNow, nil, int64(1),
	}
}

// ── GetByUserAndApplication ────────────────────────────────────

func TestRepository_GetByUserAndApplication_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetConsentByUserAndApplication)).
		WithArgs(testUserID, testAppID).
		WillReturnRows(sqlmock.NewRows(consentCols()).AddRow(newConsentRow()...))

	r := NewRepository(db)
	c, err := r.GetByUserAndApplication(context.Background(), testUserID, testAppID)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, testConsentID, c.ID)
	assert.Equal(t, []string{"openid", "profile"}, c.GrantedScopes)
	assert.Equal(t, "alice", c.Username)
	assert.Equal(t, int64(1), c.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByUserAndApplication_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetConsentByUserAndApplication)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(consentCols()))

	r := NewRepository(db)
	c, err := r.GetByUserAndApplication(context.Background(), "bad-user", "bad-app")
	assert.Error(t, err)
	assert.Nil(t, c)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── CountActiveByApplication ────────────────────────────────────

func TestRepository_CountActiveByApplication(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryCountActiveConsentsByApplication)).
		WithArgs(testAppID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(5)))

	r := NewRepository(db)
	count, err := r.CountActiveByApplication(context.Background(), testAppID)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Upsert ────────────────────────────────────────────────────────

func TestRepository_Upsert(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	grantedJSON := `["openid","profile","email"]`

	mock.ExpectExec(regexp.QuoteMeta(queryUpsertConsent)).
		WithArgs(
			testConsentID, testUserID, "alice", testOrgID, "my-org",
			testAppID, "my-app",
			grantedJSON,
			fixedNow, fixedNow, nil, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Upsert(context.Background(), &domain.Consent{
		ID:               testConsentID,
		UserID:           testUserID,
		Username:         "alice",
		OrganizationID:   testOrgID,
		OrganizationSlug: "my-org",
		ApplicationID:    testAppID,
		ApplicationSlug:  "my-app",
		GrantedScopes:    []string{"openid", "profile", "email"},
		CreatedAt:        fixedNow,
		UpdatedAt:        fixedNow,
		RevokedAt:        nil,
		Revision:         int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Revoke ────────────────────────────────────────────────────────

func TestRepository_Revoke_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeConsent)).
		WithArgs(testUserID, testAppID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Revoke(context.Background(), testUserID, testAppID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Revoke_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeConsent)).
		WithArgs(testUserID, testAppID).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Revoke(context.Background(), testUserID, testAppID)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
