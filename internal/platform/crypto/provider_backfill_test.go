package crypto

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"slices"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

func newBackfillTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *FieldEncryptor) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("IS_PRODUCTION", "false")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock, NewFieldEncryptor()
}

// encryptedValueArg matches a v2 envelope token bound to the row that decrypts
// back to the expected plaintext.
type encryptedValueArg struct {
	plaintext string
	encryptor *FieldEncryptor
	table     string
	column    string
	pk        string
}

func (a encryptedValueArg) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok || !isV2EnvelopeToken(stored) {
		return false
	}
	ctx := AADContext{Table: a.table, Column: a.column, PKValue: a.pk}
	opened, err := a.encryptor.DecryptEnvelopeFieldWithAAD(ctx, stored)
	return err == nil && opened == a.plaintext
}

func TestBackfillProviderColumn_EncryptsPlaintextRows(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	col := EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	stat := ProviderSecretBackfillStat{Table: col.Table, Column: col.Column}

	mock.ExpectQuery("SELECT id, client_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}).
			AddRow("p1", "secret-1").
			AddRow("p2", "secret-2"))
	mock.ExpectExec("UPDATE guliangiam.providers SET client_secret").
		WithArgs(encryptedValueArg{plaintext: "secret-1", encryptor: encryptor, table: col.Table, column: col.Column, pk: "p1"}, "p1", "secret-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE guliangiam.providers SET client_secret").
		WithArgs(encryptedValueArg{plaintext: "secret-2", encryptor: encryptor, table: col.Table, column: col.Column, pk: "p2"}, "p2", "secret-2").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id, client_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, "p2", 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}))

	require.NoError(t, backfillColumn(context.Background(), db, encryptor, col, 2, 0, &stat))
	assert.Equal(t, int64(2), stat.Scanned)
	assert.Equal(t, int64(2), stat.Encrypted)
	assert.Equal(t, int64(0), stat.Skipped)
	assert.Equal(t, int64(0), stat.Failed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillProviderColumn_SkipsCASMiss(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	col := EncryptedColumn{Table: "guliangiam.providers", Column: "password", PKColumn: "id"}
	stat := ProviderSecretBackfillStat{Table: col.Table, Column: col.Column}

	mock.ExpectQuery("SELECT id, password FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "password"}).AddRow("p1", "plain"))
	mock.ExpectExec("UPDATE guliangiam.providers SET password").
		WithArgs(sqlmock.AnyArg(), "p1", "plain").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT id, password FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, "p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "password"}))

	require.NoError(t, backfillColumn(context.Background(), db, encryptor, col, 1, 0, &stat))
	assert.Equal(t, int64(1), stat.Scanned)
	assert.Equal(t, int64(0), stat.Encrypted)
	assert.Equal(t, int64(1), stat.Skipped)
	assert.Equal(t, int64(0), stat.Failed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillProviderColumn_CountsUpdateFailure(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	col := EncryptedColumn{Table: "guliangiam.providers", Column: "app_secret", PKColumn: "id"}
	stat := ProviderSecretBackfillStat{Table: col.Table, Column: col.Column}

	mock.ExpectQuery("SELECT id, app_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "app_secret"}).AddRow("p1", "plain"))
	mock.ExpectExec("UPDATE guliangiam.providers SET app_secret").
		WithArgs(sqlmock.AnyArg(), "p1", "plain").
		WillReturnError(errors.New("boom"))
	mock.ExpectQuery("SELECT id, app_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, "p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "app_secret"}))

	require.NoError(t, backfillColumn(context.Background(), db, encryptor, col, 1, 0, &stat))
	assert.Equal(t, int64(1), stat.Failed)
	assert.Equal(t, int64(0), stat.Encrypted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillProviderSecrets_VerifyReportsRemainingPlaintext(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM guliangiam.providers WHERE client_secret").
		WithArgs(envelopeTokenSQLPattern).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))
	for i := 1; i < len(ProviderCredentialColumns); i++ {
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM").
			WithArgs(envelopeTokenSQLPattern).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	}

	stats, err := BackfillProviderSecrets(context.Background(), db, encryptor, ProviderSecretBackfillOptions{Verify: true})
	require.ErrorIs(t, err, ErrPlaintextProviderSecrets)
	require.Len(t, stats, len(ProviderCredentialColumns))
	assert.Equal(t, int64(2), stats[0].Plaintext)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillProviderSecrets_VerifyClean(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)

	for range ProviderCredentialColumns {
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM").
			WithArgs(envelopeTokenSQLPattern).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	}

	stats, err := BackfillProviderSecrets(context.Background(), db, encryptor, ProviderSecretBackfillOptions{Verify: true})
	require.NoError(t, err)
	require.Len(t, stats, len(ProviderCredentialColumns))
	for _, stat := range stats {
		assert.Zero(t, stat.Plaintext)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillProviderSecrets_DryRunDoesNotWrite(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)

	for range ProviderCredentialColumns {
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM").
			WithArgs(envelopeTokenSQLPattern).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	}

	stats, err := BackfillProviderSecrets(context.Background(), db, encryptor, ProviderSecretBackfillOptions{DryRun: true})
	require.NoError(t, err)
	require.Len(t, stats, len(ProviderCredentialColumns))
	assert.Equal(t, int64(1), stats[0].Plaintext)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillProviderSecrets_RejectsDryRunWithVerify(t *testing.T) {
	_, _, encryptor := newBackfillTestDB(t)
	_, err := BackfillProviderSecrets(context.Background(), nil, encryptor, ProviderSecretBackfillOptions{DryRun: true, Verify: true})
	require.Error(t, err)
}

// The backfill scan must recognize v2 tokens, otherwise a post-G7 row would be
// treated as plaintext and encrypted a second time.
func TestEnvelopeTokenSQLPattern_MatchesV2(t *testing.T) {
	pattern := regexp.MustCompile(envelopeTokenSQLPattern)
	assert.True(t, pattern.MatchString("v1-aes-aad:AAAA:BBBB:CCCC"))
	assert.True(t, pattern.MatchString("v1-sm4-aad:AAAA:BBBB:CCCC"))
	assert.True(t, pattern.MatchString("v1-aes:AAAA:BBBB:CCCC"))
	assert.False(t, pattern.MatchString("plaintext-secret"))
}

func TestProviderCredentialColumns_Registry(t *testing.T) {
	require.Len(t, ProviderCredentialColumns, 5)
	want := map[string]bool{
		"guliangiam.providers.client_secret":             true,
		"guliangiam.providers.client_secret2":            true,
		"guliangiam.providers.password":                  true,
		"guliangiam.providers.app_secret":                true,
		"guliangiam.notification_providers.access_token": true,
	}
	seen := map[string]bool{}
	for _, col := range ProviderCredentialColumns {
		require.NoError(t, col.Validate())
		assert.True(t, want[col.Table+"."+col.Column], "unexpected column %s.%s", col.Table, col.Column)
		seen[col.Table+"."+col.Column] = true
	}
	assert.Equal(t, want, seen)
}

func TestDefaultEncryptedColumns_CoversProviderCredentials(t *testing.T) {
	registered := make(map[string]bool, len(DefaultEncryptedColumns))
	for _, col := range DefaultEncryptedColumns {
		require.NoError(t, col.Validate())
		registered[col.Table+"."+col.Column] = true
	}
	for _, col := range ProviderCredentialColumns {
		assert.True(t, registered[col.Table+"."+col.Column],
			"%s.%s must be in DefaultEncryptedColumns so key rotation covers it", col.Table, col.Column)
	}
}

func TestBackfillMFASecrets_EncryptsPlaintextRows(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config").
		WithArgs(envelopeTokenSQLPattern, 2).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}).
			AddRow("user-1", "JBSWY3DPEHPK3PXP").
			AddRow("user-2", "KRSXG5CTMVRXEZLU"))
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs(encryptedValueArg{plaintext: "JBSWY3DPEHPK3PXP", encryptor: encryptor, table: tableUserMfaConfig, column: "totp_secret", pk: "user-1"}, "user-1", "JBSWY3DPEHPK3PXP").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs(encryptedValueArg{plaintext: "KRSXG5CTMVRXEZLU", encryptor: encryptor, table: tableUserMfaConfig, column: "totp_secret", pk: "user-2"}, "user-2", "KRSXG5CTMVRXEZLU").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config").
		WithArgs(envelopeTokenSQLPattern, "user-2", 2).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}))

	stats, err := BackfillMFASecrets(context.Background(), db, encryptor, EncryptedColumnBackfillOptions{BatchSize: 2})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(2), stats[0].Scanned)
	assert.Equal(t, int64(2), stats[0].Encrypted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillMFASecrets_VerifyReportsRemainingPlaintext(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM guliangiam.user_mfa_config").
		WithArgs(envelopeTokenSQLPattern).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	_, err := BackfillMFASecrets(context.Background(), db, encryptor, EncryptedColumnBackfillOptions{Verify: true})
	require.ErrorIs(t, err, ErrPlaintextMFASecrets)
}

func TestMFASecretColumns_AreRotatedNotProviderSwept(t *testing.T) {
	for _, col := range MFASecretColumns {
		require.NoError(t, col.Validate())
		assert.True(t, slices.Contains(DefaultEncryptedColumns, col))
		assert.False(t, slices.Contains(ProviderCredentialColumns, col))
	}
}

func TestRotateEncryptedData_RejectsPlaintext(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	encryptor := NewFieldEncryptor()

	_, changed, err := encryptor.RotateEncryptedData("still-plaintext")
	require.Error(t, err, "rotation must not silently treat plaintext as ciphertext")
	assert.False(t, changed)
}

// A token that slipped past the SQL predicate must still not be encrypted a
// second time: the Go-side guard is the last line of defence against data
// corruption.
func TestBackfillEncryptedColumns_SkipsEnvelopeRows(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	col := EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	stat := EncryptedColumnBackfillStat{Table: col.Table, Column: col.Column}

	sealed := "v1-sm4-aad:AAAA:BBBB:CCCC"
	mock.ExpectQuery("SELECT id, client_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}).AddRow("p1", sealed))
	mock.ExpectQuery("SELECT id, client_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, "p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}))

	require.NoError(t, backfillColumn(context.Background(), db, encryptor, col, 1, 0, &stat))
	assert.Equal(t, int64(1), stat.Scanned)
	assert.Equal(t, int64(1), stat.Skipped)
	assert.Zero(t, stat.Encrypted)
	assert.Zero(t, stat.Failed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// NULL and empty values mean "not configured"; they must be left as-is instead
// of being sealed into a non-empty token.
func TestBackfillEncryptedColumns_PreservesNullAndEmpty(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	col := EncryptedColumn{Table: "guliangiam.user_mfa_config", Column: "totp_secret", PKColumn: "user_id"}
	stat := EncryptedColumnBackfillStat{Table: col.Table, Column: col.Column}

	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config").
		WithArgs(envelopeTokenSQLPattern, 2).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}).
			AddRow("user-1", nil).
			AddRow("user-2", ""))

	require.NoError(t, backfillColumn(context.Background(), db, encryptor, col, 2, 0, &stat))
	assert.Zero(t, stat.Scanned)
	assert.Zero(t, stat.Encrypted)
	assert.Zero(t, stat.Failed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillEncryptedColumns_BatchBoundary(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	col := EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	stat := EncryptedColumnBackfillStat{Table: col.Table, Column: col.Column}

	mock.ExpectQuery("SELECT id, client_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}).
			AddRow("p1", "secret-1").
			AddRow("p2", "secret-2"))
	for _, row := range []struct{ pk, plain string }{{"p1", "secret-1"}, {"p2", "secret-2"}} {
		mock.ExpectExec("UPDATE guliangiam.providers SET client_secret").
			WithArgs(encryptedValueArg{plaintext: row.plain, encryptor: encryptor, table: col.Table, column: col.Column, pk: row.pk}, row.pk, row.plain).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectQuery("SELECT id, client_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, "p2", 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}).AddRow("p3", "secret-3"))
	mock.ExpectExec("UPDATE guliangiam.providers SET client_secret").
		WithArgs(encryptedValueArg{plaintext: "secret-3", encryptor: encryptor, table: col.Table, column: col.Column, pk: "p3"}, "p3", "secret-3").
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, backfillColumn(context.Background(), db, encryptor, col, 2, 0, &stat))
	assert.Equal(t, int64(3), stat.Scanned)
	assert.Equal(t, int64(3), stat.Encrypted)
	require.NoError(t, mock.ExpectationsWereMet())
}

// One bad row must not abort the batch: the remaining rows are still sealed and
// the failure is reported in the summary.
func TestBackfillEncryptedColumns_ContinuesAfterRowFailure(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	col := EncryptedColumn{Table: "guliangiam.providers", Column: "app_secret", PKColumn: "id"}
	stat := EncryptedColumnBackfillStat{Table: col.Table, Column: col.Column}

	mock.ExpectQuery("SELECT id, app_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "app_secret"}).
			AddRow("p1", "secret-1").
			AddRow("p2", "secret-2"))
	mock.ExpectExec("UPDATE guliangiam.providers SET app_secret").
		WithArgs(sqlmock.AnyArg(), "p1", "secret-1").
		WillReturnError(errors.New("boom"))
	mock.ExpectExec("UPDATE guliangiam.providers SET app_secret").
		WithArgs(encryptedValueArg{plaintext: "secret-2", encryptor: encryptor, table: col.Table, column: col.Column, pk: "p2"}, "p2", "secret-2").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id, app_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, "p2", 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "app_secret"}))

	require.NoError(t, backfillColumn(context.Background(), db, encryptor, col, 2, 0, &stat))
	assert.Equal(t, int64(2), stat.Scanned)
	assert.Equal(t, int64(1), stat.Encrypted)
	assert.Equal(t, int64(1), stat.Failed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A second run re-scans for non-envelope rows only; once every row is sealed the
// scan is empty and nothing is written again.
func TestBackfillEncryptedColumns_IsIdempotent(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	columns := []EncryptedColumn{{Table: "guliangiam.user_mfa_config", Column: "totp_secret", PKColumn: "user_id"}}

	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config").
		WithArgs(envelopeTokenSQLPattern, 1).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}).AddRow("user-1", "JBSWY3DPEHPK3PXP"))
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs(encryptedValueArg{plaintext: "JBSWY3DPEHPK3PXP", encryptor: encryptor, table: tableUserMfaConfig, column: "totp_secret", pk: "user-1"}, "user-1", "JBSWY3DPEHPK3PXP").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config").
		WithArgs(envelopeTokenSQLPattern, "user-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}))

	first, _, err := BackfillEncryptedColumns(context.Background(), db, encryptor, columns, EncryptedColumnBackfillOptions{BatchSize: 1})
	require.NoError(t, err)
	require.Len(t, first, 1)
	assert.Equal(t, int64(1), first[0].Encrypted)

	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config").
		WithArgs(envelopeTokenSQLPattern, 1).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}))

	second, _, err := BackfillEncryptedColumns(context.Background(), db, encryptor, columns, EncryptedColumnBackfillOptions{BatchSize: 1})
	require.NoError(t, err)
	require.Len(t, second, 1)
	assert.Zero(t, second[0].Scanned)
	assert.Zero(t, second[0].Encrypted)
	require.NoError(t, mock.ExpectationsWereMet(), "the second run must not update any row")
}

func TestBackfillEncryptedColumns_RequiresEncryptor(t *testing.T) {
	_, _, err := BackfillEncryptedColumns(context.Background(), nil, nil, MFASecretColumns, EncryptedColumnBackfillOptions{})
	require.Error(t, err)
}

// The generic CLI scans DefaultEncryptedColumns, so every column that historically
// stored plaintext must stay registered there.
func TestDefaultEncryptedColumns_CoverV8AndMFAPlaintextColumns(t *testing.T) {
	registered := make(map[string]bool, len(DefaultEncryptedColumns))
	for _, col := range DefaultEncryptedColumns {
		registered[col.Table+"."+col.Column] = true
	}
	for _, key := range []string{
		"guliangiam.radius_configs.shared_secret",
		"guliangiam.captcha_providers.secret_key",
		"guliangiam.certs.private_key",
		"guliangiam.certs.access_secret",
		"guliangiam.user_mfa_config.totp_secret",
	} {
		assert.True(t, registered[key],
			"%s must be in DefaultEncryptedColumns so backfill-encrypted-columns covers it", key)
	}
}

// A v0 direct-KEK token is already ciphertext. The backfill must leave it for
// reencrypt instead of sealing the old token a second time.
func TestBackfillEncryptedColumns_SkipsLegacyV0(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	col := EncryptedColumn{Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id"}
	stat := EncryptedColumnBackfillStat{Table: col.Table, Column: col.Column}
	v0 := directKEKToken(t, encryptor, "legacy-v0-secret")

	mock.ExpectQuery("SELECT id, client_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}).AddRow("p1", v0))
	mock.ExpectQuery("SELECT id, client_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, "p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}))

	require.NoError(t, backfillColumn(context.Background(), db, encryptor, col, 1, 0, &stat))
	assert.Equal(t, int64(1), stat.Scanned)
	assert.Equal(t, int64(1), stat.Skipped)
	assert.Equal(t, int64(1), stat.LegacyV0)
	assert.Zero(t, stat.Encrypted)
	require.NoError(t, mock.ExpectationsWereMet(), "a v0 token must not be encrypted again")
}

// Failed rows are queued by PK, and -max-failures only bounds the queue.
func TestBackfillEncryptedColumns_FailureQueueCap(t *testing.T) {
	db, mock, encryptor := newBackfillTestDB(t)
	col := EncryptedColumn{Table: "guliangiam.providers", Column: "app_secret", PKColumn: "id"}
	stat := EncryptedColumnBackfillStat{Table: col.Table, Column: col.Column}

	mock.ExpectQuery("SELECT id, app_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "app_secret"}).
			AddRow("p1", "plain-1").
			AddRow("p2", "plain-2"))
	mock.ExpectExec("UPDATE guliangiam.providers SET app_secret").
		WithArgs(sqlmock.AnyArg(), "p1", "plain-1").
		WillReturnError(errors.New("boom"))
	mock.ExpectExec("UPDATE guliangiam.providers SET app_secret").
		WithArgs(sqlmock.AnyArg(), "p2", "plain-2").
		WillReturnError(errors.New("boom"))
	mock.ExpectQuery("SELECT id, app_secret FROM guliangiam.providers").
		WithArgs(envelopeTokenSQLPattern, "p2", 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "app_secret"}))

	require.NoError(t, backfillColumn(context.Background(), db, encryptor, col, 2, 1, &stat))
	assert.Equal(t, int64(2), stat.Failed)
	require.Len(t, stat.Failures, 1)
	assert.Equal(t, "p1", stat.Failures[0].PK)
	assert.Equal(t, PayloadClassUnrecognized, stat.Failures[0].Class)
	assert.True(t, stat.FailuresTruncated)
	require.NoError(t, mock.ExpectationsWereMet())
}
