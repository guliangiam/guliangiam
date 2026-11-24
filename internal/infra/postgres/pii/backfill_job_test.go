package pii

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

var managedAccountsColumn = crypto.LegacySealedColumn{
	Table:        "guliangiam.user_collections",
	SourceColumn: "managed_accounts",
	TargetColumn: "managed_accounts_sealed",
	PKColumn:     "user_id",
}

const (
	managedAccountsSelect = "SELECT user_id, managed_accounts::text FROM guliangiam.user_collections " +
		"WHERE managed_accounts_sealed IS NULL AND COALESCE(managed_accounts::text, '') <> '' " +
		"ORDER BY user_id ASC LIMIT $1"
	managedAccountsSelectAfter = "SELECT user_id, managed_accounts::text FROM guliangiam.user_collections " +
		"WHERE managed_accounts_sealed IS NULL AND COALESCE(managed_accounts::text, '') <> '' " +
		"AND user_id > $1 ORDER BY user_id ASC LIMIT $2"
	managedAccountsUpdate = "UPDATE guliangiam.user_collections SET managed_accounts_sealed = $1 " +
		"WHERE user_id = $2 AND managed_accounts_sealed IS NULL"
	managedAccountsVerifySelect = "SELECT user_id, COALESCE(managed_accounts::text, ''), managed_accounts_sealed " +
		"FROM guliangiam.user_collections WHERE (COALESCE(managed_accounts::text, '') <> '' " +
		"OR managed_accounts_sealed IS NOT NULL) ORDER BY user_id ASC LIMIT $1"
)

func newPIIBackfillDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *crypto.FieldEncryptor) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock, crypto.NewFieldEncryptor()
}

type sealedValueForRow struct {
	encryptor *crypto.FieldEncryptor
	col       crypto.LegacySealedColumn
	pk        string
	plaintext string
}

func (m sealedValueForRow) Match(value driver.Value) bool {
	token, ok := value.(string)
	if !ok {
		return false
	}
	plaintext, err := m.encryptor.DecryptColumn(m.col.AAD(m.pk), token)
	return err == nil && plaintext == m.plaintext
}

func TestBackfillLegacySealedColumns_SealsLegacyRowsWithRowAAD(t *testing.T) {
	db, mock, encryptor := newPIIBackfillDB(t)

	first := `[{"account":"first"}]`
	second := `[{"account":"second"}]`
	mock.ExpectQuery(managedAccountsSelect).WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "managed_accounts"}).
			AddRow("pk-1", first).
			AddRow("pk-2", second))
	mock.ExpectExec(managedAccountsUpdate).
		WithArgs(sealedValueForRow{encryptor: encryptor, col: managedAccountsColumn, pk: "pk-1", plaintext: first}, "pk-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(managedAccountsUpdate).
		WithArgs(sealedValueForRow{encryptor: encryptor, col: managedAccountsColumn, pk: "pk-2", plaintext: second}, "pk-2").
		WillReturnResult(sqlmock.NewResult(0, 1))

	report, err := BackfillLegacySealedColumns(context.Background(), db, encryptor,
		[]crypto.LegacySealedColumn{managedAccountsColumn}, LegacySealedBackfillOptions{BatchSize: 10})
	require.NoError(t, err)
	require.Len(t, report.Columns, 1)
	assert.Equal(t, int64(2), report.Columns[0].Selected)
	assert.Equal(t, int64(2), report.Columns[0].Encrypted)
	assert.Equal(t, int64(0), report.Columns[0].Failed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillLegacySealedColumns_CASLossIsSkippedNotOverwritten(t *testing.T) {
	db, mock, encryptor := newPIIBackfillDB(t)

	mock.ExpectQuery(managedAccountsSelect).WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "managed_accounts"}).
			AddRow("pk-1", `[{"account":"first"}]`))
	// Zero affected rows means another writer already sealed the row.
	mock.ExpectExec(managedAccountsUpdate).
		WithArgs(sealedValueForRow{encryptor: encryptor, col: managedAccountsColumn, pk: "pk-1", plaintext: `[{"account":"first"}]`}, "pk-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	report, err := BackfillLegacySealedColumns(context.Background(), db, encryptor,
		[]crypto.LegacySealedColumn{managedAccountsColumn}, LegacySealedBackfillOptions{BatchSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), report.Columns[0].Selected)
	assert.Equal(t, int64(0), report.Columns[0].Encrypted)
	assert.Equal(t, int64(1), report.Columns[0].Skipped)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillLegacySealedColumns_DryRunCountsWithoutWriting(t *testing.T) {
	db, mock, _ := newPIIBackfillDB(t)

	mock.ExpectQuery(managedAccountsSelect).WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "managed_accounts"}).
			AddRow("pk-1", `[]`).
			AddRow("pk-2", `[]`))

	report, err := BackfillLegacySealedColumns(context.Background(), db, crypto.NewFieldEncryptor(),
		[]crypto.LegacySealedColumn{managedAccountsColumn}, LegacySealedBackfillOptions{BatchSize: 10, DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, int64(2), report.Columns[0].Selected)
	assert.Equal(t, int64(0), report.Columns[0].Encrypted)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillLegacySealedColumns_VerifyReportsThreeCounters(t *testing.T) {
	db, mock, encryptor := newPIIBackfillDB(t)

	sealed, err := encryptor.EncryptColumn(managedAccountsColumn.AAD("pk-1"), `[{"account":"sealed"}]`)
	require.NoError(t, err)
	mock.ExpectQuery(managedAccountsVerifySelect).WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "managed_accounts", "managed_accounts_sealed"}).
			AddRow("pk-1", `[{"account":"sealed"}]`, sealed).
			AddRow("pk-2", `[{"account":"legacy"}]`, nil).
			AddRow("pk-3", `[{"account":"broken"}]`, "not-an-envelope"))

	report, err := BackfillLegacySealedColumns(context.Background(), db, encryptor,
		[]crypto.LegacySealedColumn{managedAccountsColumn}, LegacySealedBackfillOptions{BatchSize: 10, Verify: true})
	require.NoError(t, err)
	stat := report.Columns[0]
	assert.Equal(t, int64(1), stat.VerifyEncrypted)
	assert.Equal(t, int64(1), stat.VerifyUnsealed)
	assert.Equal(t, int64(1), stat.VerifyDecryptFailed)
	assert.NoError(t, mock.ExpectationsWereMet())
}
