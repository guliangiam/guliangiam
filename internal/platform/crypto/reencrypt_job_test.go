package crypto

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

// newRotationEncryptor returns a v0-only encryptor and a rotating one (v0+v1).
func newRotationEncryptor(t *testing.T) (legacy, rotating *FieldEncryptor) {
	t.Helper()
	previous := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x5a}, 32))
	t.Setenv("ENCRYPTION_KEY", previous)
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	legacy = NewFieldEncryptor()
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", previous)
	return legacy, NewFieldEncryptor()
}

// GM-G2: rotation scans user_mfa_config keyed by user_id and counts its rows.
func TestReencryptJob_RunFor_MFAStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	legacy, rotating := newRotationEncryptor(t)
	// #nosec G101 -- deterministic fixtures, not real credentials.
	token1, err := legacy.EncryptField("JBSWY3DPEHPK3PXP")
	require.NoError(t, err)
	token2, err := legacy.EncryptField("KRSXG5CTMVRXEZLU")
	require.NoError(t, err)

	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config ORDER BY user_id ASC LIMIT 2").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}).
			AddRow("user-1", token1).
			AddRow("user-2", token2))
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs(encryptedValueArg{plaintext: "JBSWY3DPEHPK3PXP", encryptor: rotating, table: tableUserMfaConfig, column: "totp_secret", pk: "user-1"}, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs(encryptedValueArg{plaintext: "KRSXG5CTMVRXEZLU", encryptor: rotating, table: tableUserMfaConfig, column: "totp_secret", pk: "user-2"}, "user-2").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config WHERE user_id > \\$1").
		WithArgs("user-2").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}))

	job := NewReencryptJob(db, rotating, "")
	job.batchSize = 2
	stats, err := job.RunFor(context.Background(), MFASecretColumns)
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats.Scanned)
	assert.Equal(t, int64(2), stats.Updated)
	assert.True(t, stats.Completed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A v2 token already bound to the same row/KEK/algorithm is skipped, so a
// second RunFor pass produces no UPDATE.
func TestReencryptJob_RunFor_V2TokenIsCurrent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	_, rotating := newRotationEncryptor(t)
	col := EncryptedColumn{Table: tableUserMfaConfig, Column: "totp_secret", PKColumn: pkUserID}
	token, err := rotating.EncryptColumn(col.AAD("user-1"), "JBSWY3DPEHPK3PXP")
	require.NoError(t, err)
	require.True(t, isV2EnvelopeToken(token))

	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config ORDER BY user_id ASC LIMIT 500").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}).AddRow("user-1", token))

	job := NewReencryptJob(db, rotating, "")
	stats, err := job.RunFor(context.Background(), []EncryptedColumn{col})
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.Scanned)
	assert.Equal(t, int64(1), stats.Skipped)
	assert.Zero(t, stats.Updated, "no UPDATE is issued for a current v2 token")
	assert.True(t, stats.Completed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A PK that vanished between scan and update must be an error, not a silent skip.
func TestReencryptJob_UpdateRowZeroRowsIsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	col := EncryptedColumn{Table: tableUserMfaConfig, Column: "totp_secret", PKColumn: pkUserID}
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs("sealed-value", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	job := &ReencryptJob{db: db}
	require.Error(t, job.updateRow(context.Background(), col, "user-1", "sealed-value"))
	require.NoError(t, mock.ExpectationsWereMet())
}

// The cursor is keyed by the qualified table.column for resumable rotation.
func TestReencryptJob_MFACursorCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reencrypt.checkpoint")
	job := &ReencryptJob{checkpoint: NewCheckpointStore(path)}
	job.saveCursor(MFASecretColumns[0], "user-1")
	assert.Equal(t, "user-1", job.loadCursor(MFASecretColumns[0]))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "guliangiam.user_mfa_config.totp_secret=user-1")
}

// The first rotation batch used to bind an empty string to `id > $1`; PostgreSQL
// infers uuid for that parameter and rejects "". The initial batch must omit the
// predicate so rotation can start on a UUID primary key.
func TestReencryptJob_FetchBatch_InitialCursorOmitsPredicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	job := &ReencryptJob{db: db, batchSize: 2}
	col := EncryptedColumn{Table: "guliangiam.webhooks", Column: "secret", PKColumn: "id"}

	mock.ExpectQuery("SELECT id, secret FROM guliangiam.webhooks ORDER BY id ASC LIMIT 2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "secret"}).
			AddRow("pk-1", "token-1").
			AddRow("pk-2", ""))
	rows, err := job.fetchBatch(context.Background(), col, "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "pk-1", rows[0].pk)
	assert.Equal(t, "token-1", rows[0].value)

	mock.ExpectQuery("SELECT id, secret FROM guliangiam.webhooks WHERE id > \\$1").
		WithArgs("pk-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "secret"}))
	rows, err = job.fetchBatch(context.Background(), col, "pk-2")
	require.NoError(t, err)
	assert.Empty(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}
