package keys

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureArg 记录到达驱动的参数值，用于认证仓储实际持久化的密文。
type captureArg struct{ value driver.Value }

func (c *captureArg) Match(v driver.Value) bool {
	c.value = v
	return true
}

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow4  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID4 = "00000000-0000-0000-0000-000000000001"
	testKeyID  = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// keyCols: 13 列
func keyCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "key_type", "public_key", "private_key",
		"algorithm", "expires_at", "is_enabled", "created_at", "updated_at", "revision",
	}
}

func newKeyRow(t *testing.T) []driver.Value {
	t.Helper()
	sealed, err := sealKeyPrivateKey(testKeyID, "priv-key-data")
	if err != nil {
		t.Fatalf("seal test private key: %v", err)
	}
	return []driver.Value{
		testKeyID, testOrgID4, "my-key", "My Key", "RSA", "pub-key-data", sql.NullString{String: sealed, Valid: true},
		"RS256", fixedNow4, true, fixedNow4, fixedNow4, int64(1),
	}
}

func newKeyRowPlaintext() []driver.Value {
	return []driver.Value{
		testKeyID, testOrgID4, "my-key", "My Key", "RSA", "pub-key-data", sql.NullString{String: "legacy-plaintext-key", Valid: true},
		"RS256", fixedNow4, true, fixedNow4, fixedNow4, int64(1),
	}
}

func newKeyRowNoPrivate() []driver.Value {
	return []driver.Value{
		testKeyID, testOrgID4, "my-key", "My Key", "RSA", "pub-key-data", sql.NullString{Valid: false},
		"RS256", nil, true, fixedNow4, fixedNow4, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

// TestKeysByIDQueries_TenantPredicate pins the MySQL mirror predicate. The
// generated mirror has no RLS, so an id-only WHERE would be a cross-tenant
// read/update/delete.
func TestKeysByIDQueries_TenantPredicate(t *testing.T) {
	for name, query := range map[string]string{
		"get":    queryGetKeyByID,
		"update": queryUpdateKey,
		"delete": queryDeleteKey,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Contains(t, strings.ReplaceAll(query, " ", ""), "organization_id=?")
		})
	}
}

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetKeyByID)).
		WithArgs(testKeyID, testOrgID4).
		WillReturnRows(sqlmock.NewRows(keyCols()).AddRow(newKeyRow(t)...))

	r := NewRepository(db)
	k, err := r.GetByID(context.Background(), testKeyID, testOrgID4)
	assert.NoError(t, err)
	assert.NotNil(t, k)
	assert.Equal(t, testKeyID, k.ID)
	assert.Equal(t, "priv-key-data", k.PrivateKey)
	assert.Equal(t, "RS256", k.Algorithm)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_FoundNoPrivate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetKeyByID)).
		WithArgs(testKeyID, testOrgID4).
		WillReturnRows(sqlmock.NewRows(keyCols()).AddRow(newKeyRowNoPrivate()...))

	r := NewRepository(db)
	k, err := r.GetByID(context.Background(), testKeyID, testOrgID4)
	assert.NoError(t, err)
	assert.NotNil(t, k)
	assert.Empty(t, k.PrivateKey)
	assert.Nil(t, k.ExpiresAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_LegacyPlaintextFailsClosed(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetKeyByID)).
		WithArgs(testKeyID, testOrgID4).
		WillReturnRows(sqlmock.NewRows(keyCols()).AddRow(newKeyRowPlaintext()...))

	r := NewRepository(db)
	k, err := r.GetByID(context.Background(), testKeyID, testOrgID4)
	require.Error(t, err, "未回填的明文私钥不得当作可用私钥返回")
	assert.Nil(t, k)
	assert.Contains(t, err.Error(), "backfill-encrypted-columns")
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetKeyByID)).
		WithArgs("bad", testOrgID4).
		WillReturnRows(sqlmock.NewRows(keyCols()))

	r := NewRepository(db)
	k, err := r.GetByID(context.Background(), "bad", testOrgID4)
	assert.Error(t, err)
	assert.Nil(t, k)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×3 + key_type×2 + is_enabled×2 = 9
// listArgs: 9 + limit + offset = 11
func TestRepository_List_AllFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	keyType := "RSA"
	enabled := true
	countArgs := []driver.Value{testOrgID4, testOrgID4, "kw", "kw", "kw", keyType, keyType, enabled, enabled}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountKeys)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListKeys)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(keyCols()).AddRow(newKeyRowNoPrivate()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID4,
		Keyword:        "kw",
		KeyType:        &keyType,
		IsEnabled:      &enabled,
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Empty(t, items[0].PrivateKey)
	assert.Equal(t, int64(1), total)
	assert.Contains(t, queryListKeys, "NULL AS private_key")
	assert.NotContains(t, queryListKeys, "public_key, private_key")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_NilFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", nil, nil, nil, nil}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountKeys)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListKeys)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(keyCols()))

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

	captured := &captureArg{}
	mock.ExpectExec(regexp.QuoteMeta(queryInsertKey)).
		WithArgs(
			testKeyID, testOrgID4, "my-key", "My Key", "RSA", "pub-key-data",
			captured,
			"RS256", fixedNow4, true, fixedNow4, fixedNow4, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	expireAt := fixedNow4
	err := r.Create(context.Background(), &domain.Key{
		ID:             testKeyID,
		OrganizationID: testOrgID4,
		Name:           "my-key",
		DisplayName:    "My Key",
		KeyType:        "RSA",
		PublicKey:      "pub-key-data",
		PrivateKey:     "priv-key-data",
		Algorithm:      "RS256",
		ExpiresAt:      &expireAt,
		IsEnabled:      true,
		CreatedAt:      fixedNow4,
		UpdatedAt:      fixedNow4,
		Revision:       int64(1),
	})
	require.NoError(t, err)

	stored, ok := captured.value.(string)
	require.True(t, ok, "private_key 必须以字符串密文传入，实际类型 %T", captured.value)
	require.True(t, crypto.IsEnvelopeToken(stored), "Create 必须写密文")
	opened, err := openKeyPrivateKey(testKeyID, stored)
	require.NoError(t, err)
	assert.Equal(t, "priv-key-data", opened)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Create_NoPrivateKey(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertKey)).
		WithArgs(
			testKeyID, testOrgID4, "my-key", "My Key", "RSA", "pub-key-data",
			sql.NullString{Valid: false},
			"RS256", nil, true, fixedNow4, fixedNow4, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Key{
		ID:             testKeyID,
		OrganizationID: testOrgID4,
		Name:           "my-key",
		DisplayName:    "My Key",
		KeyType:        "RSA",
		PublicKey:      "pub-key-data",
		Algorithm:      "RS256",
		IsEnabled:      true,
		CreatedAt:      fixedNow4,
		UpdatedAt:      fixedNow4,
		Revision:       int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	captured := &captureArg{}
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateKey)).
		WithArgs(
			"Updated", "new-pub", captured,
			"ES256", fixedNow4, false, fixedNow4, int64(2), testKeyID, testOrgID4,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Key{
		ID:             testKeyID,
		OrganizationID: testOrgID4,
		DisplayName:    "Updated",
		PublicKey:      "new-pub",
		PrivateKey:     "new-priv",
		Algorithm:      "ES256",
		ExpiresAt:      &fixedNow4,
		IsEnabled:      false,
		UpdatedAt:      fixedNow4,
		Revision:       int64(2),
	})
	require.NoError(t, err)

	stored, ok := captured.value.(string)
	require.True(t, ok, "private_key 必须以字符串密文传入，实际类型 %T", captured.value)
	require.True(t, crypto.IsEnvelopeToken(stored), "Update 必须写密文")
	opened, err := openKeyPrivateKey(testKeyID, stored)
	require.NoError(t, err)
	assert.Equal(t, "new-priv", opened)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteKey)).
		WithArgs(testKeyID, testOrgID4).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testKeyID, testOrgID4)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteKey)).
		WithArgs(testKeyID, testOrgID4).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testKeyID, testOrgID4)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
