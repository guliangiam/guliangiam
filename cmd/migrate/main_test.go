package main

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
	"github.com/guliangiam/guliangiam/internal/platform/db"
)

// 账本行的 applied_at 必须打印成可回读的 RFC3339。历史上的字面布局
// "2026-09-01T15:04:05Z07:00" 不含参考年份，Go 把前导数字当文本，
// 运维看到的是 28286-09-10 这类伪年份。
func TestFormatLedgerLine(t *testing.T) {
	shanghai := time.FixedZone("CST", 8*60*60)
	applied := db.AppliedMigration{
		Kind:      db.KindStructure,
		Version:   14,
		Name:      "access_lifecycle_ldap_sync_tasks",
		Checksum:  "bcf3807452c17484",
		AppliedAt: time.Date(2026, 10, 28, 7, 24, 9, 0, shanghai),
	}

	got := formatLedgerLine(applied)
	want := "applied structure V14 access_lifecycle_ldap_sync_tasks checksum=bcf3807452c17484 " +
		"at=2026-10-28T07:24:09+08:00"
	if got != want {
		t.Fatalf("formatLedgerLine() = %q, want %q", got, want)
	}

	// 时间戳必须能被标准解析器读回，否则布局又一次写成了不可解析的字面量。
	stamp := got[strings.Index(got, "at=")+len("at="):]
	if reparsed, err := time.Parse(time.RFC3339, stamp); err != nil {
		t.Fatalf("timestamp %q is not RFC3339: %v", stamp, err)
	} else if !reparsed.Equal(applied.AppliedAt) {
		t.Fatalf("round-trip mismatch: %v != %v", reparsed, applied.AppliedAt)
	}
}

func newBackfillDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *crypto.FieldEncryptor) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("IS_PRODUCTION", "false")

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB, mock, crypto.NewFieldEncryptor()
}

func expectPayloadScan(mock sqlmock.Sqlmock, col crypto.EncryptedColumn, batchSize int, rows *sqlmock.Rows) {
	mock.ExpectQuery(fmt.Sprintf("SELECT %s, %s FROM %s ORDER BY %s ASC LIMIT \\$1",
		col.PKColumn, col.Column, col.Table, col.PKColumn)).
		WithArgs(batchSize).
		WillReturnRows(rows)
}

func expectProviderScans(mock sqlmock.Sqlmock, batchSize int, firstRows *sqlmock.Rows) {
	for i, col := range crypto.ProviderCredentialColumns {
		rows := sqlmock.NewRows([]string{col.PKColumn, col.Column})
		if i == 0 {
			rows = firstRows
		}
		expectPayloadScan(mock, col, batchSize, rows)
	}
}

func TestEncryptProviderSecrets_VerifyExitCode(t *testing.T) {
	col := crypto.ProviderCredentialColumns[0]
	sqlDB, mock, encryptor := newBackfillDB(t)
	expectProviderScans(mock, 200, sqlmock.NewRows([]string{col.PKColumn, col.Column}).AddRow("p1", "plain"))
	assert.Equal(t, 1, encryptProviderSecretsWithDB(context.Background(), sqlDB, encryptor,
		crypto.ProviderSecretBackfillOptions{Verify: true}))
	require.NoError(t, mock.ExpectationsWereMet())

	sqlDB2, mock2, encryptor2 := newBackfillDB(t)
	expectProviderScans(mock2, 200, sqlmock.NewRows([]string{col.PKColumn, col.Column}))
	assert.Equal(t, 0, encryptProviderSecretsWithDB(context.Background(), sqlDB2, encryptor2,
		crypto.ProviderSecretBackfillOptions{Verify: true}))
	require.NoError(t, mock2.ExpectationsWereMet())
}

func TestEncryptProviderSecrets_DryRunExitCode(t *testing.T) {
	col := crypto.ProviderCredentialColumns[0]
	sqlDB, mock, encryptor := newBackfillDB(t)
	expectProviderScans(mock, 200, sqlmock.NewRows([]string{col.PKColumn, col.Column}).
		AddRow("p1", "plain-1").
		AddRow("p2", "plain-2").
		AddRow("p3", "plain-3"))
	assert.Equal(t, 0, encryptProviderSecretsWithDB(context.Background(), sqlDB, encryptor,
		crypto.ProviderSecretBackfillOptions{DryRun: true}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEncryptProviderSecrets_FailedRowsExitCode(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)

	mock.ExpectQuery("SELECT id, client_secret FROM").
		WithArgs(sqlmock.AnyArg(), 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}).AddRow("p1", "plain"))
	mock.ExpectExec("UPDATE guliangiam.providers SET client_secret").
		WithArgs(sqlmock.AnyArg(), "p1", "plain").
		WillReturnError(sqlmock.ErrCancelled)
	for _, column := range []string{"client_secret2", "password", "app_secret"} {
		mock.ExpectQuery("SELECT id, "+column+" FROM").
			WithArgs(sqlmock.AnyArg(), 2).
			WillReturnRows(sqlmock.NewRows([]string{"id", column}))
	}
	mock.ExpectQuery("SELECT id, access_token FROM").
		WithArgs(sqlmock.AnyArg(), 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "access_token"}))

	assert.Equal(t, 1, encryptProviderSecretsWithDB(context.Background(), sqlDB, encryptor,
		crypto.ProviderSecretBackfillOptions{BatchSize: 2}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEncryptProviderSecrets_RejectsInvalidFlags(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	assert.Equal(t, 2, runEncryptProviderSecrets([]string{"-dry-run", "-verify"}))
	assert.Equal(t, 2, runEncryptProviderSecrets([]string{"-batch-size", "0"}))
	assert.Equal(t, 2, runEncryptProviderSecrets([]string{"-max-failures", "-1"}))
	assert.Equal(t, 2, runWithArgs([]string{"unknown-command"}))
}

func TestEncryptProviderSecrets_FailsWithoutKey(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "postgres://example.invalid/db")
	assert.Equal(t, 1, runEncryptProviderSecrets(nil))
}

// The reencrypt command must be dispatched and must fail before opening a pool
// when no KEK is configured: a connection attempt to the invalid DSN would
// return a different error path (and could hang instead of failing fast).
func TestReencrypt_FailsWithoutKeyBeforeConnecting(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 1, runWithArgs([]string{"reencrypt", "-dsn", "postgres://example.invalid/db"}))
}

// With a KEK but no DSN the command is recognized and reaches flag/DSN
// validation (exit 2), proving it is no longer an "unknown command".
func TestReencrypt_CommandIsRecognized(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 2, runWithArgs([]string{"reencrypt"}))
}

func TestBackfillEncryptedColumns_CommandIsRecognized(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 2, runWithArgs([]string{"backfill-encrypted-columns"}))
}

func TestBackfillEncryptedColumns_FailsWithoutKeyBeforeConnecting(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 1, runBackfillEncryptedColumns([]string{"-dsn", "postgres://example.invalid/db"}))
}

func TestBackfillEncryptedColumns_RejectsInvalidFlags(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	assert.Equal(t, 2, runBackfillEncryptedColumns([]string{"-dry-run", "-verify"}))
	assert.Equal(t, 2, runBackfillEncryptedColumns([]string{"-batch-size", "0"}))
	assert.Equal(t, 2, runBackfillEncryptedColumns([]string{"-max-failures", "-1"}))
}

func TestBackfillEncryptedColumns_VerifyExitCode(t *testing.T) {
	col := crypto.EncryptedColumn{Table: "guliangiam.user_mfa_config", Column: "totp_secret", PKColumn: "user_id"}
	columns := []crypto.EncryptedColumn{col}

	sqlDB, mock, encryptor := newBackfillDB(t)
	expectPayloadScan(mock, col, 200, sqlmock.NewRows([]string{col.PKColumn, col.Column}).AddRow("user-1", "plaintext-seed"))
	assert.Equal(t, 1, backfillEncryptedColumnsWithDB(context.Background(), sqlDB, encryptor, columns, nil,
		crypto.EncryptedColumnBackfillOptions{Verify: true}))
	require.NoError(t, mock.ExpectationsWereMet())

	sqlDB2, mock2, encryptor2 := newBackfillDB(t)
	expectPayloadScan(mock2, col, 200, sqlmock.NewRows([]string{col.PKColumn, col.Column}))
	assert.Equal(t, 0, backfillEncryptedColumnsWithDB(context.Background(), sqlDB2, encryptor2, columns, nil,
		crypto.EncryptedColumnBackfillOptions{Verify: true}))
	require.NoError(t, mock2.ExpectationsWereMet())
}

func TestBackfillEncryptedColumns_DryRunExitCode(t *testing.T) {
	col := crypto.EncryptedColumn{Table: "guliangiam.certs", Column: "private_key", PKColumn: "id"}
	sqlDB, mock, encryptor := newBackfillDB(t)
	expectPayloadScan(mock, col, 200, sqlmock.NewRows([]string{col.PKColumn, col.Column}).
		AddRow("cert-1", "plain-1").
		AddRow("cert-2", "plain-2").
		AddRow("cert-3", "plain-3"))
	assert.Equal(t, 0, backfillEncryptedColumnsWithDB(context.Background(), sqlDB, encryptor,
		[]crypto.EncryptedColumn{col}, nil, crypto.EncryptedColumnBackfillOptions{DryRun: true}))
	require.NoError(t, mock.ExpectationsWereMet())
}

// A row that cannot be encrypted is reported in the summary and turns the exit
// code non-zero, but it must not stop the batch.
func TestBackfillEncryptedColumns_FailedRowsExitCode(t *testing.T) {
	col := crypto.EncryptedColumn{Table: "guliangiam.user_mfa_config", Column: "totp_secret", PKColumn: "user_id"}
	sqlDB, mock, encryptor := newBackfillDB(t)

	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config").
		WithArgs(sqlmock.AnyArg(), 2).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}).
			AddRow("user-1", "JBSWY3DPEHPK3PXP").
			AddRow("user-2", "KRSXG5CTMVRXEZLU"))
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs(sqlmock.AnyArg(), "user-1", "JBSWY3DPEHPK3PXP").
		WillReturnError(sqlmock.ErrCancelled)
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs(sqlmock.AnyArg(), "user-2", "KRSXG5CTMVRXEZLU").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config").
		WithArgs(sqlmock.AnyArg(), "user-2", 2).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}))

	out := captureStdout(t, func() {
		assert.Equal(t, 1, backfillEncryptedColumnsWithDB(context.Background(), sqlDB, encryptor,
			[]crypto.EncryptedColumn{col}, nil, crypto.EncryptedColumnBackfillOptions{BatchSize: 2}))
	})
	assert.Contains(t, out, "FAIL guliangiam.user_mfa_config.totp_secret pk=user-1 class=unrecognized stage=update")
	assert.Contains(t, out, "failures_shown=1")
	require.NoError(t, mock.ExpectationsWereMet())
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	fn()
	require.NoError(t, w.Close())
	os.Stdout = old
	return <-done
}

func legacyV1Token(t *testing.T, encryptor *crypto.FieldEncryptor, plaintext string) string {
	t.Helper()
	token, err := encryptor.EncryptField(plaintext)
	require.NoError(t, err)
	require.True(t, crypto.IsEnvelopeToken(token))
	require.False(t, strings.HasSuffix(strings.SplitN(token, ":", 2)[0], "-aad"), "fixture must be v1, not v2")
	return token
}

// Backfill verify must not silently classify a v1 envelope as plaintext.
func TestBackfillEncryptedColumns_VerifyClassifiesLegacyV1(t *testing.T) {
	col := crypto.EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	sqlDB, mock, encryptor := newBackfillDB(t)
	v1 := legacyV1Token(t, encryptor, "legacy-secret")

	expectPayloadScan(mock, col, 10, sqlmock.NewRows([]string{col.PKColumn, col.Column}).
		AddRow("p1", "plaintext-secret").
		AddRow("p2", v1))

	var code int
	out := captureStdout(t, func() {
		code = backfillEncryptedColumnsWithDB(context.Background(), sqlDB, encryptor,
			[]crypto.EncryptedColumn{col}, nil, crypto.EncryptedColumnBackfillOptions{Verify: true, BatchSize: 10, MaxFailures: 1})
	})
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "legacy-v1=1")
	assert.Contains(t, out, "FAIL guliangiam.providers.client_secret pk=p1 class=unrecognized")
	assert.Contains(t, out, "truncated=false", "only fatal rows consume the queue")
	assert.NotContains(t, out, "pk=p2 class=legacy-v1", "legacy rows are counted, not queued")
	assert.NotContains(t, out, "plaintext-secret", "the queue must never print the stored plaintext")
	assert.NotContains(t, out, v1, "the queue must never print the stored ciphertext")
	require.NoError(t, mock.ExpectationsWereMet())
}

// A v1 envelope is encrypted, just not current: backfill verify exits 0 and
// tells the operator to run reencrypt instead of sealing it again.
func TestBackfillEncryptedColumns_VerifyLegacyOnlyExitsZero(t *testing.T) {
	col := crypto.EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	sqlDB, mock, encryptor := newBackfillDB(t)
	v1 := legacyV1Token(t, encryptor, "legacy-secret")

	expectPayloadScan(mock, col, 10, sqlmock.NewRows([]string{col.PKColumn, col.Column}).AddRow("p1", v1))

	var code int
	out := captureStdout(t, func() {
		code = backfillEncryptedColumnsWithDB(context.Background(), sqlDB, encryptor,
			[]crypto.EncryptedColumn{col}, nil, crypto.EncryptedColumnBackfillOptions{Verify: true, BatchSize: 10})
	})
	assert.Equal(t, 0, code)
	assert.NotContains(t, out, "LEGACY ", "advisory legacy rows are counted, not queued")
	assert.Contains(t, out, "legacy-v1=1")
	assert.Contains(t, out, "run `migrate reencrypt`")
	assert.Contains(t, out, "verify OK")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReencryptVerify_QueueClassifiesV1AndUnrecognized(t *testing.T) {
	col := crypto.EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	sqlDB, mock, encryptor := newBackfillDB(t)
	v1 := legacyV1Token(t, encryptor, "legacy-secret")
	current, err := encryptor.EncryptColumn(col.AAD("p3"), "current-secret")
	require.NoError(t, err)

	expectPayloadScan(mock, col, 500, sqlmock.NewRows([]string{col.PKColumn, col.Column}).
		AddRow("p1", v1).
		AddRow("p2", "plaintext-secret").
		AddRow("p3", current).
		AddRow("p4", nil).
		AddRow("p5", ""))

	var code int
	out := captureStdout(t, func() {
		code = reencryptColumnsFor(context.Background(), sqlDB, encryptor, []crypto.EncryptedColumn{col},
			reencryptOptions{Verify: true, MaxFailures: 2})
	})
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "scanned=5 skipped=2 current-v2=1")
	assert.Contains(t, out, "FAIL guliangiam.providers.client_secret pk=p1 class=v1-legacy")
	assert.Contains(t, out, "FAIL guliangiam.providers.client_secret pk=p2 class=unrecognized")
	assert.NotContains(t, out, "pk=p3", "a current v2 row is not queued")
	assert.Contains(t, out, "verify summary: scanned=5 skipped=2 current-v2=1 issues=2 failures=2 shown=2 truncated=false")
	assert.NotContains(t, out, "plaintext-secret", "the queue must never print the stored plaintext")
	assert.NotContains(t, out, v1, "the queue must never print the stored ciphertext")
	assert.NotContains(t, out, current, "the queue must never print the stored ciphertext")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReencryptVerify_DoesNotTouchCheckpoint(t *testing.T) {
	col := crypto.EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	sqlDB, mock, encryptor := newBackfillDB(t)
	expectPayloadScan(mock, col, 500, sqlmock.NewRows([]string{col.PKColumn, col.Column}))

	path := filepath.Join(t.TempDir(), "reencrypt.checkpoint")
	require.NoError(t, os.WriteFile(path, []byte("guliangiam.providers.client_secret=p1\n"), 0o600))
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	var code int
	_ = captureStdout(t, func() {
		code = reencryptColumnsFor(context.Background(), sqlDB, encryptor, []crypto.EncryptedColumn{col},
			reencryptOptions{Verify: true, Checkpoint: path})
	})
	assert.Equal(t, 0, code)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after, "verify must not read or write the resume checkpoint")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReencryptDryRun_DoesNotWriteOrCreateCheckpoint(t *testing.T) {
	col := crypto.EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	sqlDB, mock, encryptor := newBackfillDB(t)
	v1 := legacyV1Token(t, encryptor, "legacy-secret")
	expectPayloadScan(mock, col, 500, sqlmock.NewRows([]string{col.PKColumn, col.Column}).AddRow("p1", v1))

	path := filepath.Join(t.TempDir(), "reencrypt.checkpoint")
	var code int
	out := captureStdout(t, func() {
		code = reencryptColumnsFor(context.Background(), sqlDB, encryptor, []crypto.EncryptedColumn{col},
			reencryptOptions{DryRun: true, Checkpoint: path})
	})
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "would-update=1")
	assert.NotContains(t, out, "verify OK", "dry-run must not masquerade as verify")
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "dry-run must not create a checkpoint")
	require.NoError(t, mock.ExpectationsWereMet(), "dry-run must issue no UPDATE")
}

// A truncated queue must keep counting failures even when the printed keys are
// capped.
func TestBackfillEncryptedColumns_VerifyTruncatedQueueStillFails(t *testing.T) {
	col := crypto.EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	sqlDB, mock, encryptor := newBackfillDB(t)

	expectPayloadScan(mock, col, 10, sqlmock.NewRows([]string{col.PKColumn, col.Column}).
		AddRow("p1", "plaintext-1").
		AddRow("p2", "plaintext-2").
		AddRow("p3", "plaintext-3"))

	var code int
	out := captureStdout(t, func() {
		code = backfillEncryptedColumnsWithDB(context.Background(), sqlDB, encryptor,
			[]crypto.EncryptedColumn{col}, nil, crypto.EncryptedColumnBackfillOptions{Verify: true, BatchSize: 10, MaxFailures: 1})
	})
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "FAIL guliangiam.providers.client_secret pk=p1 class=unrecognized")
	assert.NotContains(t, out, "pk=p2 class=unrecognized", "the queue is capped")
	assert.Contains(t, out, "failures=3 shown=1 truncated=true")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReencryptVerify_TruncatedQueueStillFails(t *testing.T) {
	col := crypto.EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	sqlDB, mock, encryptor := newBackfillDB(t)
	expectPayloadScan(mock, col, 500, sqlmock.NewRows([]string{col.PKColumn, col.Column}).
		AddRow("p1", "plain-1").
		AddRow("p2", "plain-2").
		AddRow("p3", "plain-3"))

	var code int
	out := captureStdout(t, func() {
		code = reencryptColumnsFor(context.Background(), sqlDB, encryptor, []crypto.EncryptedColumn{col},
			reencryptOptions{Verify: true, MaxFailures: 1})
	})
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "FAIL guliangiam.providers.client_secret pk=p1 class=unrecognized")
	assert.NotContains(t, out, "pk=p2")
	assert.Contains(t, out, "failures=3 shown=1 truncated=true")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReencrypt_RejectsInvalidFlags(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	assert.Equal(t, 2, runReencrypt([]string{"-dry-run", "-verify"}))
	assert.Equal(t, 2, runReencrypt([]string{"-max-failures", "-1"}))
}

// newSealedRotationDB returns a v0-only encryptor (stale token fixture) and a
// rotating one (current v1 + previous v0), mirroring the crypto package helper.
func newSealedRotationDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *crypto.FieldEncryptor, *crypto.FieldEncryptor) {
	t.Helper()
	previous := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x5a}, 32))
	t.Setenv("ENCRYPTION_KEY", previous)
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("IS_PRODUCTION", "false")
	legacy := crypto.NewFieldEncryptor()
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", previous)
	rotating := crypto.NewFieldEncryptor()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB, mock, legacy, rotating
}

// sealedPayloadV1Arg matches a re-sealed value that is a v1 envelope (never
// v2/AAD) and opens back to the expected plaintext with DecryptField.
type sealedPayloadV1Arg struct {
	plaintext string
	encryptor *crypto.FieldEncryptor
}

func (a sealedPayloadV1Arg) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok || !crypto.IsEnvelopeToken(stored) {
		return false
	}
	header, _, _ := strings.Cut(stored, ":")
	if strings.HasSuffix(header, "-aad") {
		return false
	}
	opened, err := a.encryptor.DecryptField(stored)
	return err == nil && opened == a.plaintext
}

// A sealed payload must be re-sealed as v1: upgrading it to v2 would make the
// application's DecryptField reads fail with ErrAADContext.
func TestReencryptSealedPayloads_ResealsV1AndStaysV1(t *testing.T) {
	sqlDB, mock, legacy, rotating := newSealedRotationDB(t)
	col := crypto.SealedPayloadColumns[0]
	stale, err := legacy.EncryptField("sk-sealed")
	require.NoError(t, err)
	require.True(t, crypto.IsEnvelopeToken(stale))

	mock.ExpectQuery("SELECT id, api_key_sealed FROM guliangiam.llm_providers ORDER BY id ASC LIMIT 500").
		WillReturnRows(sqlmock.NewRows([]string{"id", "api_key_sealed"}).AddRow("p1", stale))
	mock.ExpectExec("UPDATE guliangiam.llm_providers SET api_key_sealed").
		WithArgs(sealedPayloadV1Arg{plaintext: "sk-sealed", encryptor: rotating}, "p1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	path := filepath.Join(t.TempDir(), "reencrypt.checkpoint")
	var code int
	out := captureStdout(t, func() {
		code = reencryptTargetsFor(context.Background(), sqlDB, rotating, nil, []crypto.SealedPayloadColumn{col},
			reencryptOptions{Checkpoint: path})
	})
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "api_key_sealed (sealed payload): scanned=1 updated=1 skipped=0 errors=0")
	assert.Contains(t, out, "summary: scanned=1 updated=1 skipped=0 errors=0")
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "a successful run clears the checkpoint")
	require.NoError(t, mock.ExpectationsWereMet())
}

// A v2 token in a sealed payload column cannot be opened by DecryptField. It
// must be reported as unrecognized and never rewritten or downgraded.
func TestReencryptSealedPayloads_RefusesV2WithoutWriting(t *testing.T) {
	sqlDB, mock, _, rotating := newSealedRotationDB(t)
	col := crypto.SealedPayloadColumns[0]
	v2, err := rotating.EncryptColumn(crypto.AADContext{Table: col.Table, Column: col.Column, PKValue: "p1"}, "sk-sealed")
	require.NoError(t, err)
	require.Contains(t, v2, "-aad:")

	mock.ExpectQuery("SELECT id, api_key_sealed FROM guliangiam.llm_providers ORDER BY id ASC LIMIT 500").
		WillReturnRows(sqlmock.NewRows([]string{"id", "api_key_sealed"}).AddRow("p1", v2))

	path := filepath.Join(t.TempDir(), "reencrypt.checkpoint")
	var code int
	out := captureStdout(t, func() {
		code = reencryptTargetsFor(context.Background(), sqlDB, rotating, nil, []crypto.SealedPayloadColumn{col},
			reencryptOptions{Checkpoint: path})
	})
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "FAIL guliangiam.llm_providers.api_key_sealed pk=p1 class=unrecognized stage=reseal")
	assert.NotContains(t, out, v2, "the queue must never print the stored ciphertext")
	assert.NotContains(t, out, "sk-sealed", "the queue must never print the stored plaintext")
	require.NoError(t, mock.ExpectationsWereMet(), "a v2 token must not produce an UPDATE")
}

// Empty/NULL means "not configured" and must stay untouched.
func TestReencryptSealedPayloads_SkipsEmptyValues(t *testing.T) {
	sqlDB, mock, _, rotating := newSealedRotationDB(t)
	col := crypto.SealedPayloadColumns[0]

	mock.ExpectQuery("SELECT id, api_key_sealed FROM guliangiam.llm_providers ORDER BY id ASC LIMIT 500").
		WillReturnRows(sqlmock.NewRows([]string{"id", "api_key_sealed"}).
			AddRow("p1", nil).
			AddRow("p2", ""))

	path := filepath.Join(t.TempDir(), "reencrypt.checkpoint")
	var code int
	out := captureStdout(t, func() {
		code = reencryptTargetsFor(context.Background(), sqlDB, rotating, nil, []crypto.SealedPayloadColumn{col},
			reencryptOptions{Checkpoint: path})
	})
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "scanned=0 updated=0 skipped=0 errors=0")
	require.NoError(t, mock.ExpectationsWereMet(), "empty values must not produce an UPDATE")
}

// backfill -verify must fail on plaintext in a sealed payload column even
// though the write path of this command never seals those columns (they stay
// v1 and are rotated by `migrate reencrypt`).
func TestBackfillEncryptedColumns_VerifySealedPlaintext(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	col := crypto.SealedPayloadColumns[0]
	expectPayloadScan(mock, crypto.EncryptedColumn{Table: col.Table, Column: col.Column, PKColumn: col.PKColumn}, 10,
		sqlmock.NewRows([]string{"id", "api_key_sealed"}).AddRow("p1", "sk-plaintext"))

	var code int
	out := captureStdout(t, func() {
		code = backfillEncryptedColumnsWithDB(context.Background(), sqlDB, encryptor, nil,
			[]crypto.SealedPayloadColumn{col}, crypto.EncryptedColumnBackfillOptions{Verify: true, BatchSize: 10})
	})
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "FAIL guliangiam.llm_providers.api_key_sealed (sealed payload) pk=p1 class=unrecognized")
	assert.Contains(t, out, "sealed payload summary: scanned=1 skipped=0 legacy-v1=0 legacy-v0=0 unrecognized=1 unreadable=0 failures=1")
	assert.NotContains(t, out, "sk-plaintext", "the queue must never print the stored plaintext")
	require.NoError(t, mock.ExpectationsWereMet())
}

// backfill -dry-run only classifies: it must report sealed payloads without
// writing and exit 0.
func TestBackfillEncryptedColumns_DryRunSealedPayloads(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	col := crypto.SealedPayloadColumns[0]
	expectPayloadScan(mock, crypto.EncryptedColumn{Table: col.Table, Column: col.Column, PKColumn: col.PKColumn}, 10,
		sqlmock.NewRows([]string{"id", "api_key_sealed"}).AddRow("p1", "sk-plaintext"))

	var code int
	out := captureStdout(t, func() {
		code = backfillEncryptedColumnsWithDB(context.Background(), sqlDB, encryptor, nil,
			[]crypto.SealedPayloadColumn{col}, crypto.EncryptedColumnBackfillOptions{DryRun: true, BatchSize: 10})
	})
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "sealed payload summary: scanned=1 skipped=0 legacy-v1=0 legacy-v0=0 unrecognized=1 unreadable=0 (dry-run, no rows written)")
	assert.Contains(t, out, "this backfill does not seal them (they must stay v1)")
	assert.NotContains(t, out, "sk-plaintext")
	require.NoError(t, mock.ExpectationsWereMet())
}

// -max-failures caps the printed queue but the summary still counts every row.
func TestReencryptSealedPayloads_MaxFailuresQueue(t *testing.T) {
	sqlDB, mock, _, rotating := newSealedRotationDB(t)
	col := crypto.SealedPayloadColumns[1]

	mock.ExpectQuery("SELECT id, config_sealed FROM guliangiam.pay_channel_configs ORDER BY id ASC LIMIT 500").
		WillReturnRows(sqlmock.NewRows([]string{"id", "config_sealed"}).
			AddRow("p1", "plain-1").
			AddRow("p2", "plain-2").
			AddRow("p3", "plain-3"))

	path := filepath.Join(t.TempDir(), "reencrypt.checkpoint")
	var code int
	out := captureStdout(t, func() {
		code = reencryptTargetsFor(context.Background(), sqlDB, rotating, nil, []crypto.SealedPayloadColumn{col},
			reencryptOptions{Checkpoint: path, MaxFailures: 1})
	})
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "FAIL guliangiam.pay_channel_configs.config_sealed pk=p1 class=unrecognized stage=reseal")
	assert.NotContains(t, out, "pk=p2", "the queue is capped")
	assert.Contains(t, out, "errors=3 failures_shown=1")
	assert.Contains(t, out, "truncated=true")
	require.NoError(t, mock.ExpectationsWereMet(), "plaintext must not produce an UPDATE")
}
