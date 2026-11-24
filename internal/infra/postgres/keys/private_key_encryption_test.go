package keys

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

const testPrivateKeyMaterial = "-----BEGIN PRIVATE KEY-----\ntest-fixture\n-----END PRIVATE KEY-----"

func TestKeyPrivateKey_SealOpenRoundTrip(t *testing.T) {
	sealed, err := sealKeyPrivateKey("key-1", testPrivateKeyMaterial)
	require.NoError(t, err)
	require.NotEqual(t, testPrivateKeyMaterial, sealed)
	require.True(t, crypto.IsEnvelopeToken(sealed), "写入必须是信封令牌")

	opened, err := openKeyPrivateKey("key-1", sealed)
	require.NoError(t, err)
	assert.Equal(t, testPrivateKeyMaterial, opened)

	// The envelope is bound to the key row: another id must not authenticate it.
	_, err = openKeyPrivateKey("key-2", sealed)
	require.Error(t, err)
}

func TestKeyPrivateKey_EmptyAndNullStayEmpty(t *testing.T) {
	sealed, err := sealKeyPrivateKey("key-1", "")
	require.NoError(t, err, "空私钥表示未配置，不应产生密文")
	assert.Empty(t, sealed)

	opened, err := openKeyPrivateKey("key-1", "")
	require.NoError(t, err)
	assert.Empty(t, opened)
}

func TestKeyPrivateKey_LegacyPlaintextFailsClosed(t *testing.T) {
	opened, err := openKeyPrivateKey("key-legacy", testPrivateKeyMaterial)
	require.Error(t, err, "未回填的明文不得按可用私钥返回")
	assert.Empty(t, opened)
	assert.Contains(t, err.Error(), "backfill-encrypted-columns")
}

func TestRepository_Create_SealsPrivateKey(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
			captured = args
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	k := &domain.Key{ID: "key-1", OrganizationID: "org-1", PrivateKey: testPrivateKeyMaterial}

	require.NoError(t, NewRepository(db).Create(context.Background(), k))
	require.Len(t, captured, 13)

	stored, ok := captured[6].(*string)
	require.True(t, ok, "非空私钥必须写入密文，实际类型 %T", captured[6])
	require.True(t, crypto.IsEnvelopeToken(*stored))

	opened, err := openKeyPrivateKey(k.ID, *stored)
	require.NoError(t, err)
	assert.Equal(t, testPrivateKeyMaterial, opened)
	assert.Equal(t, testPrivateKeyMaterial, k.PrivateKey, "加密不得修改 domain 对象")
}

func TestRepository_Create_EmptyPrivateKeyStaysNull(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
			captured = args
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}

	require.NoError(t, NewRepository(db).Create(context.Background(), &domain.Key{ID: "key-1"}))
	require.Len(t, captured, 13)
	stored, ok := captured[6].(*string)
	require.True(t, ok, "nullable 列必须以 *string 传入，实际类型 %T", captured[6])
	assert.Nil(t, stored, "空私钥保持 SQL NULL")
}

func TestRepository_Update_SealsPrivateKey(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
			captured = args
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}

	k := &domain.Key{ID: "key-1", OrganizationID: "org-1", PrivateKey: testPrivateKeyMaterial}
	require.NoError(t, NewRepository(db).Update(context.Background(), k))
	require.Len(t, captured, 10)

	stored, ok := captured[2].(*string)
	require.True(t, ok, "非空私钥必须写入密文，实际类型 %T", captured[2])
	require.True(t, crypto.IsEnvelopeToken(*stored))
	opened, err := openKeyPrivateKey(k.ID, *stored)
	require.NoError(t, err)
	assert.Equal(t, testPrivateKeyMaterial, opened)
}

// captureArg records the value that reached the driver while accepting any
// value, so the test can authenticate the ciphertext the repository persisted.
type captureArg struct{ value driver.Value }

func (c *captureArg) Match(v driver.Value) bool {
	c.value = v
	return true
}

// TestKeyPrivateKey_BackfillThenRead exercises the operator backfill on the
// exact keys column: a legacy plaintext row is encrypted in place, and the
// repository read path opens the resulting envelope.
func TestKeyPrivateKey_BackfillThenRead(t *testing.T) {
	ctx := context.Background()
	pk := "00000000-0000-0000-0000-0000000000aa"
	legacyPlaintext := testPrivateKeyMaterial

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT id, private_key FROM guliangiam.keys WHERE private_key <> '' AND private_key !~ $1 ORDER BY id ASC LIMIT $2`,
	)).
		WithArgs(sqlmock.AnyArg(), 200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "private_key"}).AddRow(pk, legacyPlaintext))

	captured := &captureArg{}
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE guliangiam.keys SET private_key = $1 WHERE id = $2 AND private_key = $3`,
	)).
		WithArgs(captured, pk, legacyPlaintext).
		WillReturnResult(sqlmock.NewResult(0, 1))

	columns := []crypto.EncryptedColumn{{Table: keyTable, Column: keyColumnPrivateKey, PKColumn: "id"}}
	stats, plaintextRemaining, err := crypto.BackfillEncryptedColumns(ctx, db, crypto.GlobalFieldEncryptor(), columns, crypto.EncryptedColumnBackfillOptions{})
	require.NoError(t, err)
	require.False(t, plaintextRemaining)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(1), stats[0].Scanned)
	assert.Equal(t, int64(1), stats[0].Encrypted)

	sealed, ok := captured.value.(string)
	require.True(t, ok, "回填写入值必须是字符串，实际类型 %T", captured.value)
	require.True(t, crypto.IsEnvelopeToken(sealed))

	now := time.Now()
	sc := &mockScanner{values: []interface{}{
		pk, "org-1", "legacy-key", "Legacy Key",
		"RSA", "public-key-data", &sealed,
		"RS256", nil,
		true, now, now, int64(1),
	}}
	k, err := scanKey(sc)
	require.NoError(t, err, "回填后的行必须可读")
	assert.Equal(t, legacyPlaintext, k.PrivateKey)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT COUNT(*) FROM guliangiam.keys WHERE private_key <> '' AND private_key !~ $1`,
	)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	stats, plaintextRemaining, err = crypto.BackfillEncryptedColumns(ctx, db, crypto.GlobalFieldEncryptor(), columns,
		crypto.EncryptedColumnBackfillOptions{Verify: true})
	require.NoError(t, err)
	require.False(t, plaintextRemaining)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(0), stats[0].Plaintext)
	require.NoError(t, mock.ExpectationsWereMet())
}
