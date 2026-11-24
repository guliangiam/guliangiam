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

// sealedCredentialArg matches a written value that is a v2 envelope bound to the
// row id and opens back to the expected plaintext.
type sealedCredentialArg struct {
	encryptor *crypto.FieldEncryptor
	column    crypto.EncryptedColumn
	plaintext string
	pk        string
}

func (a sealedCredentialArg) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok || !crypto.IsEnvelopeToken(stored) {
		return false
	}
	header, _, _ := strings.Cut(stored, ":")
	if !strings.HasSuffix(header, "-aad") {
		return false
	}
	opened, err := a.encryptor.DecryptColumn(a.column.AAD(a.pk), stored)
	return err == nil && opened == a.plaintext
}

func credentialSecretColumnsForTest() []crypto.EncryptedColumn {
	return crypto.CredentialSecretColumns
}

func expectCredentialSecretScan(mock sqlmock.Sqlmock, col crypto.EncryptedColumn, args []driver.Value, cursor string, rows *sqlmock.Rows) {
	if cursor == "" {
		mock.ExpectQuery(regexp.QuoteMeta(
			fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s <> '' AND %s !~ $1 ORDER BY %s ASC LIMIT $2",
				col.PKColumn, col.Column, col.Table, col.Column, col.Column, col.PKColumn))).
			WithArgs(args...).WillReturnRows(rows)
		return
	}
	mock.ExpectQuery(regexp.QuoteMeta(
		fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s <> '' AND %s !~ $1 AND %s > $2 ORDER BY %s ASC LIMIT $3",
			col.PKColumn, col.Column, col.Table, col.Column, col.Column, col.PKColumn, col.PKColumn))).
		WithArgs(args...).WillReturnRows(rows)
}

func expectCredentialSecretCount(mock sqlmock.Sqlmock, col crypto.EncryptedColumn, count int64) {
	mock.ExpectQuery(regexp.QuoteMeta(
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s <> '' AND %s !~ $1", col.Table, col.Column, col.Column))).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
}

func credentialSecretRows(col crypto.EncryptedColumn, values ...driver.Value) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{col.PKColumn, col.Column})
	for i := 0; i+1 < len(values); i += 2 {
		rows.AddRow(values[i], values[i+1])
	}
	return rows
}

func TestEncryptCredentialSecrets_SealsPlaintextAndSkipsEnvelope(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	columns := credentialSecretColumnsForTest()
	mcpCol, idvCol := columns[0], columns[1]
	alreadySealed, err := encryptor.EncryptColumn(mcpCol.AAD("p2"), "tok-sealed")
	require.NoError(t, err)

	expectCredentialSecretScan(mock, mcpCol, []driver.Value{sqlmock.AnyArg(), 2}, "",
		credentialSecretRows(mcpCol, "p1", "tok-plain", "p2", alreadySealed))
	mock.ExpectExec(regexp.QuoteMeta(
		fmt.Sprintf("UPDATE %s SET %s = $1 WHERE %s = $2 AND %s = $3", mcpCol.Table, mcpCol.Column, mcpCol.PKColumn, mcpCol.Column))).
		WithArgs(sealedCredentialArg{encryptor: encryptor, column: mcpCol, plaintext: "tok-plain", pk: "p1"}, "p1", "tok-plain").
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectCredentialSecretScan(mock, mcpCol, []driver.Value{sqlmock.AnyArg(), "p2", 2}, "p2", credentialSecretRows(mcpCol))

	expectCredentialSecretScan(mock, idvCol, []driver.Value{sqlmock.AnyArg(), 2}, "",
		credentialSecretRows(idvCol, "p3", "sess-plain"))
	mock.ExpectExec(regexp.QuoteMeta(
		fmt.Sprintf("UPDATE %s SET %s = $1 WHERE %s = $2 AND %s = $3", idvCol.Table, idvCol.Column, idvCol.PKColumn, idvCol.Column))).
		WithArgs(sealedCredentialArg{encryptor: encryptor, column: idvCol, plaintext: "sess-plain", pk: "p3"}, "p3", "sess-plain").
		WillReturnResult(sqlmock.NewResult(0, 1))

	var stats credentialSecretsStats
	var code int
	out := captureStdout(t, func() {
		stats, code = encryptCredentialSecretsWithDB(context.Background(), sqlDB, encryptor, columns,
			credentialSecretsOptions{BatchSize: 2})
	})
	assert.Equal(t, 0, code)
	assert.Equal(t, int64(3), stats.Scanned)
	assert.Equal(t, int64(2), stats.Encrypted)
	assert.Equal(t, int64(1), stats.Skipped, "an already-sealed row must never be rewritten")
	assert.Equal(t, int64(0), stats.Failed)
	assert.Contains(t, out, "guliangiam.mcp_servers.token: selected=2 encrypted=1 skipped=1")
	assert.Contains(t, out, "guliangiam.identity_verifications.session_token: selected=1 encrypted=1 skipped=0")
	assert.Contains(t, out, "summary: selected=3 encrypted=2 skipped=1 failed=0")
	assert.NotContains(t, out, "tok-plain")
	assert.NotContains(t, out, "sess-plain")
	assert.NotContains(t, out, alreadySealed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEncryptCredentialSecrets_SecondRunIsNoop(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	columns := credentialSecretColumnsForTest()
	for _, col := range columns {
		expectCredentialSecretScan(mock, col, []driver.Value{sqlmock.AnyArg(), 2}, "", credentialSecretRows(col))
	}

	var stats credentialSecretsStats
	var code int
	out := captureStdout(t, func() {
		stats, code = encryptCredentialSecretsWithDB(context.Background(), sqlDB, encryptor, columns,
			credentialSecretsOptions{BatchSize: 2})
	})
	assert.Equal(t, 0, code)
	assert.Equal(t, int64(0), stats.Scanned)
	assert.Equal(t, int64(0), stats.Encrypted)
	assert.Contains(t, out, "summary: selected=0 encrypted=0 skipped=0 failed=0 legacy_v1=0 legacy_v0=0 columns=2")
	require.NoError(t, mock.ExpectationsWereMet(), "a re-run over sealed rows issues no UPDATE")
}

func TestEncryptCredentialSecrets_FailedRowDoesNotRollBackProcessedRows(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	columns := credentialSecretColumnsForTest()
	mcpCol, idvCol := columns[0], columns[1]

	expectCredentialSecretScan(mock, mcpCol, []driver.Value{sqlmock.AnyArg(), 2}, "",
		credentialSecretRows(mcpCol, "p1", "tok-one", "p2", "tok-two"))
	mock.ExpectExec(regexp.QuoteMeta(
		fmt.Sprintf("UPDATE %s SET %s = $1 WHERE %s = $2 AND %s = $3", mcpCol.Table, mcpCol.Column, mcpCol.PKColumn, mcpCol.Column))).
		WithArgs(sealedCredentialArg{encryptor: encryptor, column: mcpCol, plaintext: "tok-one", pk: "p1"}, "p1", "tok-one").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(
		fmt.Sprintf("UPDATE %s SET %s = $1 WHERE %s = $2 AND %s = $3", mcpCol.Table, mcpCol.Column, mcpCol.PKColumn, mcpCol.Column))).
		WithArgs(sqlmock.AnyArg(), "p2", "tok-two").
		WillReturnError(sqlmock.ErrCancelled)
	expectCredentialSecretScan(mock, mcpCol, []driver.Value{sqlmock.AnyArg(), "p2", 2}, "p2", credentialSecretRows(mcpCol))
	expectCredentialSecretScan(mock, idvCol, []driver.Value{sqlmock.AnyArg(), 2}, "", credentialSecretRows(idvCol))

	var stats credentialSecretsStats
	var code int
	out := captureStdout(t, func() {
		stats, code = encryptCredentialSecretsWithDB(context.Background(), sqlDB, encryptor, columns,
			credentialSecretsOptions{BatchSize: 2})
	})
	assert.Equal(t, 1, code, "a failed row must fail the run")
	assert.Equal(t, int64(1), stats.Encrypted, "the row committed before the failure must stay processed")
	assert.Equal(t, int64(1), stats.Failed)
	assert.Contains(t, out, "encrypted=1")
	assert.Contains(t, out, "failed=1")
	require.NoError(t, mock.ExpectationsWereMet(), "no transaction/rollback may wrap the row updates")
}

func TestEncryptCredentialSecrets_VerifyExitCode(t *testing.T) {
	columns := credentialSecretColumnsForTest()

	sqlDB, mock, encryptor := newBackfillDB(t)
	expectCredentialSecretCount(mock, columns[0], 2)
	expectCredentialSecretCount(mock, columns[1], 0)
	assert.Equal(t, 1, encryptCredentialSecretsWithDBExpectCode(t, sqlDB, encryptor, columns,
		credentialSecretsOptions{Verify: true}), "plaintext remaining must fail verify")
	require.NoError(t, mock.ExpectationsWereMet())

	sqlDB2, mock2, encryptor2 := newBackfillDB(t)
	for _, col := range columns {
		expectCredentialSecretCount(mock2, col, 0)
	}
	assert.Equal(t, 0, encryptCredentialSecretsWithDBExpectCode(t, sqlDB2, encryptor2, columns,
		credentialSecretsOptions{Verify: true}))
	require.NoError(t, mock2.ExpectationsWereMet())
}

func TestEncryptCredentialSecrets_DryRunCountsWithoutWriting(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	columns := credentialSecretColumnsForTest()
	expectCredentialSecretCount(mock, columns[0], 1)
	expectCredentialSecretCount(mock, columns[1], 2)

	var code int
	out := captureStdout(t, func() {
		code = encryptCredentialSecretsWithDBExpectCode(t, sqlDB, encryptor, columns, credentialSecretsOptions{DryRun: true})
	})
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "guliangiam.mcp_servers.token: would_encrypt=1")
	assert.Contains(t, out, "guliangiam.identity_verifications.session_token: would_encrypt=2")
	assert.Contains(t, out, "summary: would_encrypt=3 columns=2")
	assert.NotContains(t, out, "UPDATE")
	require.NoError(t, mock.ExpectationsWereMet(), "dry-run issues no UPDATE")
}

func encryptCredentialSecretsWithDBExpectCode(t *testing.T, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, columns []crypto.EncryptedColumn, opts credentialSecretsOptions) int {
	t.Helper()
	_, code := encryptCredentialSecretsWithDB(context.Background(), sqlDB, encryptor, columns, opts)
	return code
}

func TestEncryptCredentialSecrets_RejectsInvalidFlags(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	assert.Equal(t, 2, runEncryptCredentialSecrets([]string{"-dry-run", "-verify"}))
	assert.Equal(t, 2, runEncryptCredentialSecrets([]string{"-batch-size", "0"}))
	assert.Equal(t, 2, runEncryptCredentialSecrets([]string{"-max-failures", "-1"}))
}

func TestEncryptCredentialSecrets_FailsWithoutKeyBeforeConnecting(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 1, runEncryptCredentialSecrets([]string{"-dsn", "postgres://example.invalid/db"}))
}

func TestEncryptCredentialSecrets_CommandIsRecognized(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 2, runWithArgs([]string{"encrypt-credential-secrets"}))
}
