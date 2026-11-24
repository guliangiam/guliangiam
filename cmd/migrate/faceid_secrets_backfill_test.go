package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

// sealedFaceIdSecretArg matches a written value that is a v2 envelope bound to
// the row id and opens back to the expected plaintext.
type sealedFaceIdSecretArg struct {
	encryptor *crypto.FieldEncryptor
	plaintext string
	provider  string
}

func (a sealedFaceIdSecretArg) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok || !crypto.IsEnvelopeToken(stored) {
		return false
	}
	header, _, _ := strings.Cut(stored, ":")
	if !strings.HasSuffix(header, "-aad") {
		return false
	}
	opened, err := a.encryptor.DecryptColumn(
		crypto.AADContext{Table: faceIdSecretTable, Column: faceIdSecretColumn, PKValue: a.provider},
		stored,
	)
	return err == nil && opened == a.plaintext
}

func expectFaceIdSecretScan(mock sqlmock.Sqlmock, args []driver.Value, cursor string, rows *sqlmock.Rows) {
	if cursor == "" {
		mock.ExpectQuery(regexp.QuoteMeta(
			fmt.Sprintf("SELECT id, access_key_secret FROM guliangiam.face_id_providers WHERE access_key_secret <> '' AND access_key_secret !~ $1 ORDER BY id ASC LIMIT $2"))).
			WithArgs(args...).WillReturnRows(rows)
		return
	}
	mock.ExpectQuery(regexp.QuoteMeta(
		fmt.Sprintf("SELECT id, access_key_secret FROM guliangiam.face_id_providers WHERE access_key_secret <> '' AND access_key_secret !~ $1 AND id > $2 ORDER BY id ASC LIMIT $3"))).
		WithArgs(args...).WillReturnRows(rows)
}

func expectFaceIdSecretCount(mock sqlmock.Sqlmock, count int64) {
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT COUNT(*) FROM guliangiam.face_id_providers WHERE access_key_secret <> '' AND access_key_secret !~ $1")).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
}

func faceIdSecretRows(values ...driver.Value) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "access_key_secret"})
	for i := 0; i+1 < len(values); i += 2 {
		rows.AddRow(values[i], values[i+1])
	}
	return rows
}

func TestEncryptFaceIDSecrets_SealsPlaintextAndSkipsEnvelope(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	alreadySealed, err := encryptor.EncryptColumn(
		crypto.AADContext{Table: faceIdSecretTable, Column: faceIdSecretColumn, PKValue: "p2"}, "sk-sealed")
	require.NoError(t, err)

	expectFaceIdSecretScan(mock, []driver.Value{sqlmock.AnyArg(), 2}, "",
		faceIdSecretRows("p1", "sk-plain", "p2", alreadySealed))
	mock.ExpectExec(regexp.QuoteMeta(
		"UPDATE guliangiam.face_id_providers SET access_key_secret = $1 WHERE id = $2 AND access_key_secret = $3")).
		WithArgs(sealedFaceIdSecretArg{encryptor: encryptor, plaintext: "sk-plain", provider: "p1"}, "p1", "sk-plain").
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectFaceIdSecretScan(mock, []driver.Value{sqlmock.AnyArg(), "p2", 2}, "p2", faceIdSecretRows())

	var stats faceIdSecretsStats
	var code int
	out := captureStdout(t, func() {
		stats, code = encryptFaceIDSecretsWithDB(context.Background(), sqlDB, encryptor,
			faceIdSecretsOptions{BatchSize: 2})
	})
	assert.Equal(t, 0, code)
	assert.Equal(t, int64(2), stats.Scanned)
	assert.Equal(t, int64(1), stats.Encrypted)
	assert.Equal(t, int64(1), stats.Skipped, "an already-sealed row must never be rewritten")
	assert.Equal(t, int64(0), stats.Failed)
	assert.Contains(t, out, "encrypted=1")
	assert.Contains(t, out, "skipped=1")
	assert.NotContains(t, out, "sk-plain", "the command must never print stored values")
	assert.NotContains(t, out, alreadySealed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEncryptFaceIDSecrets_SecondRunIsNoop(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	expectFaceIdSecretScan(mock, []driver.Value{sqlmock.AnyArg(), 2}, "", faceIdSecretRows())

	var stats faceIdSecretsStats
	var code int
	out := captureStdout(t, func() {
		stats, code = encryptFaceIDSecretsWithDB(context.Background(), sqlDB, encryptor,
			faceIdSecretsOptions{BatchSize: 2})
	})
	assert.Equal(t, 0, code)
	assert.Equal(t, int64(0), stats.Scanned)
	assert.Equal(t, int64(0), stats.Encrypted)
	assert.Contains(t, out, "summary: selected=0 encrypted=0 skipped=0 failed=0")
	require.NoError(t, mock.ExpectationsWereMet(), "a re-run over sealed rows issues no UPDATE")
}

func TestEncryptFaceIDSecrets_FailedRowDoesNotRollBackProcessedRows(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)

	expectFaceIdSecretScan(mock, []driver.Value{sqlmock.AnyArg(), 2}, "",
		faceIdSecretRows("p1", "sk-one", "p2", "sk-two"))
	mock.ExpectExec(regexp.QuoteMeta(
		"UPDATE guliangiam.face_id_providers SET access_key_secret = $1 WHERE id = $2 AND access_key_secret = $3")).
		WithArgs(sealedFaceIdSecretArg{encryptor: encryptor, plaintext: "sk-one", provider: "p1"}, "p1", "sk-one").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(
		"UPDATE guliangiam.face_id_providers SET access_key_secret = $1 WHERE id = $2 AND access_key_secret = $3")).
		WithArgs(sqlmock.AnyArg(), "p2", "sk-two").
		WillReturnError(sqlmock.ErrCancelled)
	expectFaceIdSecretScan(mock, []driver.Value{sqlmock.AnyArg(), "p2", 2}, "p2", faceIdSecretRows())

	var stats faceIdSecretsStats
	var code int
	out := captureStdout(t, func() {
		stats, code = encryptFaceIDSecretsWithDB(context.Background(), sqlDB, encryptor,
			faceIdSecretsOptions{BatchSize: 2})
	})
	assert.Equal(t, 1, code, "a failed row must fail the run")
	assert.Equal(t, int64(1), stats.Encrypted, "the row committed before the failure must stay processed")
	assert.Equal(t, int64(1), stats.Failed)
	assert.Contains(t, out, "encrypted=1")
	assert.Contains(t, out, "failed=1")
	require.NoError(t, mock.ExpectationsWereMet(), "no transaction/rollback may wrap the row updates")
}

func TestEncryptFaceIDSecrets_VerifyExitCode(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	expectFaceIdSecretCount(mock, 2)
	assert.Equal(t, 1, encryptFaceIDSecretsWithDBExpectCode(t, sqlDB, encryptor,
		faceIdSecretsOptions{Verify: true}), "plaintext remaining must fail verify")
	require.NoError(t, mock.ExpectationsWereMet())

	sqlDB2, mock2, encryptor2 := newBackfillDB(t)
	expectFaceIdSecretCount(mock2, 0)
	assert.Equal(t, 0, encryptFaceIDSecretsWithDBExpectCode(t, sqlDB2, encryptor2,
		faceIdSecretsOptions{Verify: true}))
	require.NoError(t, mock2.ExpectationsWereMet())
}

func TestEncryptFaceIDSecrets_DryRunCountsWithoutWriting(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	expectFaceIdSecretCount(mock, 3)
	var code int
	out := captureStdout(t, func() {
		code = encryptFaceIDSecretsWithDBExpectCode(t, sqlDB, encryptor, faceIdSecretsOptions{DryRun: true})
	})
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "would_encrypt=3")
	assert.NotContains(t, out, "UPDATE")
	require.NoError(t, mock.ExpectationsWereMet(), "dry-run issues no UPDATE")
}

func encryptFaceIDSecretsWithDBExpectCode(t *testing.T, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, opts faceIdSecretsOptions) int {
	t.Helper()
	_, code := encryptFaceIDSecretsWithDB(context.Background(), sqlDB, encryptor, opts)
	return code
}

func TestEncryptFaceIDSecrets_RejectsInvalidFlags(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	assert.Equal(t, 2, runEncryptFaceIDSecrets([]string{"-dry-run", "-verify"}))
	assert.Equal(t, 2, runEncryptFaceIDSecrets([]string{"-batch-size", "0"}))
	assert.Equal(t, 2, runEncryptFaceIDSecrets([]string{"-max-failures", "-1"}))
}

func TestEncryptFaceIDSecrets_FailsWithoutKeyBeforeConnecting(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 1, runEncryptFaceIDSecrets([]string{"-dsn", "postgres://example.invalid/db"}))
}

func TestEncryptFaceIDSecrets_CommandIsRecognized(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 2, runWithArgs([]string{"encrypt-faceid-secrets"}))
}
