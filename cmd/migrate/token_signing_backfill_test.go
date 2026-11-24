package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTokenSigningDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB, mock
}
func expectTokenSigningBatch(mock sqlmock.Sqlmock, cursor string, batchSize int, rows *sqlmock.Rows) {
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(tokenSigningRLSBypassSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(tokenSigningScanSQL)).WithArgs(cursor, batchSize).WillReturnRows(rows)
}

func tokenSigningRows(values ...driver.Value) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"application_id", tokenSigningColumn})
	for i := 0; i+1 < len(values); i += 2 {
		rows.AddRow(values[i], values[i+1])
	}
	return rows
}
func TestTokenSigningBackfill_DryRunClassifiesWithoutWriting(t *testing.T) {
	sqlDB, mock := newTokenSigningDB(t)
	expectTokenSigningBatch(mock, tokenSigningZeroUUID, 2, tokenSigningRows(
		"app-1", "", "app-2", nil,
	))
	mock.ExpectCommit()
	expectTokenSigningBatch(mock, "app-2", 2, tokenSigningRows(
		"app-3", "  ", "app-4", "RS256",
	))
	mock.ExpectCommit()
	stats, code := backfillApplicationTokenSigningWithDB(context.Background(), sqlDB, tokenSigningOptions{BatchSize: 2, DryRun: true})
	assert.Equal(t, 1, code, "NULL rows require manual investigation")
	assert.Equal(t, int64(4), stats.Scanned)
	assert.Equal(t, int64(1), stats.Null)
	assert.Equal(t, int64(1), stats.Empty)
	assert.Equal(t, int64(1), stats.Whitespace)
	assert.Equal(t, int64(1), stats.Valid)
	assert.Equal(t, int64(0), stats.Updated, "dry-run never writes")
	assert.Equal(t, int64(2), stats.blank())
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestTokenSigningBackfill_WriteBatchesUpdateOnlyBlankRows(t *testing.T) {
	sqlDB, mock := newTokenSigningDB(t)
	expectTokenSigningBatch(mock, tokenSigningZeroUUID, 2, tokenSigningRows(
		"app-1", "", "app-2", "  ",
	))
	query, _ := tokenSigningUpdateStatement([]string{"app-1", "app-2"})
	assert.Contains(t, query, "revision = revision + 1")
	mock.ExpectExec(regexp.QuoteMeta(query)).WithArgs("app-1", "app-2").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	expectTokenSigningBatch(mock, "app-2", 2, tokenSigningRows(
		"app-3", "RS256", "app-4", "ES256",
	))
	mock.ExpectCommit()
	expectTokenSigningBatch(mock, "app-4", 2, tokenSigningRows("app-5", "SM2-SM3"))
	mock.ExpectCommit()

	stats, code := backfillApplicationTokenSigningWithDB(context.Background(), sqlDB, tokenSigningOptions{BatchSize: 2})
	assert.Equal(t, 1, code, "out-of-enum values must fail the run")
	assert.Equal(t, int64(5), stats.Scanned)
	assert.Equal(t, int64(2), stats.Updated)
	assert.Equal(t, int64(1), stats.Invalid)
	assert.Equal(t, int64(2), stats.Valid)
	require.NoError(t, mock.ExpectationsWereMet(), "invalid rows are never updated")
}

func TestTokenSigningBackfill_VerifyPassAndFail(t *testing.T) {
	sqlDB, mock := newTokenSigningDB(t)
	expectTokenSigningBatch(mock, tokenSigningZeroUUID, 3, tokenSigningRows("app-1", "", "app-2", "ES256"))
	mock.ExpectCommit()
	stats, code := backfillApplicationTokenSigningWithDB(context.Background(), sqlDB, tokenSigningOptions{BatchSize: 3, Verify: true})
	assert.Equal(t, 1, code)
	assert.Equal(t, int64(1), stats.blank())
	assert.Equal(t, int64(1), stats.Invalid)
	assert.Equal(t, int64(0), stats.Updated)
	require.NoError(t, mock.ExpectationsWereMet())

	sqlDB2, mock2 := newTokenSigningDB(t)
	expectTokenSigningBatch(mock2, tokenSigningZeroUUID, 3, tokenSigningRows("app-1", "RS256", "app-2", "SM2-SM3"))
	mock2.ExpectCommit()
	_, code = backfillApplicationTokenSigningWithDB(context.Background(), sqlDB2, tokenSigningOptions{BatchSize: 3, Verify: true})
	assert.Equal(t, 0, code)
	require.NoError(t, mock2.ExpectationsWereMet())
}

func TestTokenSigningBackfill_SecondRunIsNoop(t *testing.T) {
	sqlDB, mock := newTokenSigningDB(t)
	expectTokenSigningBatch(mock, tokenSigningZeroUUID, 2, tokenSigningRows("app-1", "", "app-2", "RS256"))
	query, _ := tokenSigningUpdateStatement([]string{"app-1"})
	mock.ExpectExec(regexp.QuoteMeta(query)).WithArgs("app-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectTokenSigningBatch(mock, "app-2", 2, tokenSigningRows("app-3", "SM2-SM3"))
	mock.ExpectCommit()
	stats, code := backfillApplicationTokenSigningWithDB(context.Background(), sqlDB, tokenSigningOptions{BatchSize: 2})
	assert.Equal(t, 0, code)
	assert.Equal(t, int64(1), stats.Updated)
	require.NoError(t, mock.ExpectationsWereMet())

	sqlDB2, mock2 := newTokenSigningDB(t)
	expectTokenSigningBatch(mock2, tokenSigningZeroUUID, 3, tokenSigningRows("app-1", "RS256", "app-2", "SM2-SM3"))
	mock2.ExpectCommit()
	stats, code = backfillApplicationTokenSigningWithDB(context.Background(), sqlDB2, tokenSigningOptions{BatchSize: 3})
	assert.Equal(t, 0, code)
	assert.Equal(t, int64(0), stats.Updated, "second run has nothing to normalize")
	require.NoError(t, mock2.ExpectationsWereMet())
}

func TestBackfillApplicationTokenSigning_RejectsInvalidFlags(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 2, runWithArgs([]string{"backfill-application-token-signing"}))
	assert.Equal(t, 2, runBackfillApplicationTokenSigning([]string{"-dry-run", "-verify"}))
	assert.Equal(t, 2, runBackfillApplicationTokenSigning([]string{"-batch-size", "0"}))
}
