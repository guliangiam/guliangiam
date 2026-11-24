package authorizationcodes

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow    = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedExpiry = time.Date(2026, 9, 24, 10, 5, 0, 0, time.UTC)
	fixedAuth   = time.Date(2026, 9, 24, 9, 55, 0, 0, time.UTC)
	testCodeID  = "00000000-0000-0000-0000-000000000001"
	testUserID  = "00000000-0000-0000-0000-000000000002"
	testOrgID   = "00000000-0000-0000-0000-000000000003"
	testAppID   = "00000000-0000-0000-0000-000000000004"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义（19 列，严格匹配 GetByCode SELECT 顺序）────────────────

func codeCols() []string {
	return []string{
		"id", "user_id", "username", "organization_id", "organization_slug",
		"application_id", "application_slug", "redirect_uri", "scopes", "state",
		"nonce", "code_challenge", "code_challenge_method", "resource", "sid",
		"auth_time", "expires_at", "used_at", "created_at",
	}
}

// newCodeRow 严格匹配 authorizationCodeRow 的 Scan 顺序（19 列）。
func newCodeRow() []driver.Value {
	return []driver.Value{
		testCodeID, testUserID, "alice", testOrgID, "my-org",
		testAppID, "my-app", "https://rp.example/cb",
		`["openid","profile"]`, "xyzstate", "nonce-abc",
		"S256challengehashxxx", "S256", "urn:example:resource", "sid-001",
		fixedAuth, fixedExpiry, nil, fixedNow,
	}
}

// ── GetByCode ────────────────────────────────────────────────────

func TestRepository_GetByCode_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	plaintext := "auth-code-plain-xyz"
	codeHash := crypto.HashTokenValue(plaintext)

	mock.ExpectQuery(regexp.QuoteMeta(queryGetAuthorizationCodeByCode)).
		WithArgs(codeHash).
		WillReturnRows(sqlmock.NewRows(codeCols()).AddRow(newCodeRow()...))

	r := NewRepository(db)
	item, err := r.GetByCode(context.Background(), plaintext)
	assert.NoError(t, err)
	assert.NotNil(t, item)
	assert.Equal(t, testCodeID, item.ID)
	assert.Equal(t, testUserID, item.UserID)
	assert.Equal(t, "alice", item.Username)
	assert.Equal(t, []string{"openid", "profile"}, item.Scopes)
	assert.Empty(t, item.Code) // 明文从不回填
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByCode_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetAuthorizationCodeByCode)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(codeCols()))

	r := NewRepository(db)
	item, err := r.GetByCode(context.Background(), "unknown-code")
	assert.Error(t, err)
	assert.Nil(t, item)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	scopesJSON, _ := json.Marshal([]string{"openid", "profile", "email"})

	mock.ExpectExec(regexp.QuoteMeta(queryInsertAuthorizationCode)).
		WithArgs(
			testCodeID,
			sqlmock.AnyArg(), // code_hash 由 crypto.HashTokenValue 生成
			sqlmock.AnyArg(), // code_prefix 取明文前 8 字节
			testUserID, testAppID,
			"https://rp.example/cb",
			string(scopesJSON),
			"xyzstate", "sid-001", "nonce-abc",
			"S256challengehashxxx", "S256", "urn:example:resource",
			fixedAuth, fixedExpiry, fixedNow,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	plaintext := "auth-code-plain-xyz"
	err := r.Create(context.Background(), &domain.AuthorizationCode{
		ID:              testCodeID,
		Code:            plaintext,
		UserID:          testUserID,
		ApplicationID:   testAppID,
		RedirectURI:     "https://rp.example/cb",
		Scopes:          []string{"openid", "profile", "email"},
		State:           "xyzstate",
		Sid:             "sid-001",
		Nonce:           "nonce-abc",
		CodeChallenge:   "S256challengehashxxx",
		ChallengeMethod: "S256",
		Resource:        "urn:example:resource",
		AuthTime:        fixedAuth,
		ExpiresAt:       fixedExpiry,
		CreatedAt:       fixedNow,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── MarkUsed ──────────────────────────────────────────────────────

func TestRepository_MarkUsed_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	usedAt := fixedNow.Add(time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(queryMarkAuthorizationCodeUsed)).
		WithArgs(usedAt, testCodeID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.MarkUsed(context.Background(), testCodeID, usedAt)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_MarkUsed_AlreadyConsumed(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	usedAt := fixedNow.Add(time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(queryMarkAuthorizationCodeUsed)).
		WithArgs(usedAt, testCodeID).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 affected rows

	r := NewRepository(db)
	err := r.MarkUsed(context.Background(), testCodeID, usedAt)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
