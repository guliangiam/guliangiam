package cryptopayload

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	firstPageSQL = "SELECT id, client_secret FROM guliangiam.providers ORDER BY id ASC LIMIT $1"
	keysetSQL    = "SELECT id, client_secret FROM guliangiam.providers WHERE id > $1 ORDER BY id ASC LIMIT $2"
)

var providerBatch = crypto.PayloadBatch{
	Table: "guliangiam.providers", Column: "client_secret", PKColumn: "id", Limit: 10,
}

func newSourceDB(t *testing.T) (sqlmock.Sqlmock, Source) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return mock, NewSource(db)
}

// The first page must keep the historical template exactly: no cursor
// predicate, parameterized LIMIT.
func TestFetchPayloadBatch_FirstPageSQL(t *testing.T) {
	mock, source := newSourceDB(t)
	mock.ExpectQuery(firstPageSQL).WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}).
			AddRow("p1", "secret").
			AddRow("p2", nil).
			AddRow("p3", ""))

	rows, err := source.FetchPayloadBatch(context.Background(), providerBatch)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, "p1", rows[0].PK)
	assert.Equal(t, "secret", rows[0].Value.String)
	assert.False(t, rows[1].Value.Valid, "NULL stays distinguishable from an empty string")
	assert.Equal(t, "p3", rows[2].PK)
	assert.Equal(t, "", rows[2].Value.String)
	assert.True(t, rows[2].Value.Valid)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A non-empty cursor must use the keyset predicate and the second LIMIT
// parameter, preserving the original ordering and pagination semantics.
func TestFetchPayloadBatch_KeysetPageSQL(t *testing.T) {
	mock, source := newSourceDB(t)
	batch := providerBatch
	batch.AfterPK = "p2"
	mock.ExpectQuery(keysetSQL).WithArgs("p2", 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}).AddRow("p3", "secret"))

	rows, err := source.FetchPayloadBatch(context.Background(), batch)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "p3", rows[0].PK)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchPayloadBatch_NullPrimaryKeyIsRejected(t *testing.T) {
	mock, source := newSourceDB(t)
	mock.ExpectQuery(firstPageSQL).WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "client_secret"}).AddRow(nil, "secret"))

	_, err := source.FetchPayloadBatch(context.Background(), providerBatch)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInternal, appErr.Code)
	assert.Contains(t, appErr.Message, "guliangiam.providers.client_secret row has NULL primary key")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchPayloadBatch_QueryErrorIsWrapped(t *testing.T) {
	mock, source := newSourceDB(t)
	cause := errors.New("database unavailable")
	mock.ExpectQuery(firstPageSQL).WithArgs(10).WillReturnError(cause)

	_, err := source.FetchPayloadBatch(context.Background(), providerBatch)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInternal, appErr.Code)
	assert.Equal(t, "scan guliangiam.providers.client_secret", appErr.Message)
	assert.ErrorIs(t, err, cause)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchPayloadBatch_ScanErrorIsWrapped(t *testing.T) {
	mock, source := newSourceDB(t)
	mock.ExpectQuery(firstPageSQL).WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("p1"))

	_, err := source.FetchPayloadBatch(context.Background(), providerBatch)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInternal, appErr.Code)
	assert.Equal(t, "scan guliangiam.providers.client_secret", appErr.Message)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchPayloadBatch_RowIterationErrorIsWrapped(t *testing.T) {
	mock, source := newSourceDB(t)
	cause := errors.New("row iteration failed")
	rows := sqlmock.NewRows([]string{"id", "client_secret"}).
		AddRow("p1", "secret").
		RowError(0, cause)
	mock.ExpectQuery(firstPageSQL).WithArgs(10).WillReturnRows(rows)

	_, err := source.FetchPayloadBatch(context.Background(), providerBatch)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInternal, appErr.Code)
	assert.Equal(t, "scan guliangiam.providers.client_secret", appErr.Message)
	assert.ErrorIs(t, err, cause)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchPayloadBatch_RejectsUnsafeIdentifiers(t *testing.T) {
	mock, source := newSourceDB(t)
	batch := providerBatch
	batch.Table = "guliangiam.providers; DROP TABLE"

	_, err := source.FetchPayloadBatch(context.Background(), batch)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Contains(t, appErr.Message, "unsafe table identifier")
	require.NoError(t, mock.ExpectationsWereMet(), "an unsafe identifier must never reach the database")
}

func TestUpdateSealedPayload_CompareAndSwapSQL(t *testing.T) {
	mock, source := newSourceDB(t)
	col := crypto.SealedPayloadColumn{
		Table: "guliangiam.pay_channel_configs", Column: "config_sealed", PKColumn: "id",
	}
	mock.ExpectExec("UPDATE guliangiam.pay_channel_configs SET config_sealed = $1 WHERE id = $2 AND config_sealed = $3").
		WithArgs("sealed-token", "c1", "plaintext").
		WillReturnResult(sqlmock.NewResult(0, 1))

	affected, err := source.UpdateSealedPayload(context.Background(), crypto.SealedPayloadWrite{
		Column: col, PK: "c1", Expected: "plaintext", Sealed: "sealed-token",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSealedPayload_ZeroAffectedRowsIsLostRace(t *testing.T) {
	mock, source := newSourceDB(t)
	col := crypto.SealedPayloadColumn{
		Table: "guliangiam.llm_providers", Column: "api_key_sealed", PKColumn: "id",
	}
	mock.ExpectExec("UPDATE guliangiam.llm_providers SET api_key_sealed = $1 WHERE id = $2 AND api_key_sealed = $3").
		WithArgs("sealed-token", "p1", "plaintext").
		WillReturnResult(sqlmock.NewResult(0, 0))

	affected, err := source.UpdateSealedPayload(context.Background(), crypto.SealedPayloadWrite{
		Column: col, PK: "p1", Expected: "plaintext", Sealed: "sealed-token",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), affected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSealedPayload_RejectsUnsafeIdentifiers(t *testing.T) {
	mock, source := newSourceDB(t)
	col := crypto.SealedPayloadColumn{
		Table: "guliangiam.pay_channel_configs; DROP TABLE", Column: "config_sealed", PKColumn: "id",
	}
	_, err := source.UpdateSealedPayload(context.Background(), crypto.SealedPayloadWrite{
		Column: col, PK: "c1", Expected: "plaintext", Sealed: "sealed-token",
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	require.NoError(t, mock.ExpectationsWereMet(), "an unsafe identifier must never reach the database")
}

func TestUpdateSealedPayload_RejectsEmptyReplacement(t *testing.T) {
	mock, source := newSourceDB(t)
	col := crypto.SealedPayloadColumn{
		Table: "guliangiam.pay_channel_configs", Column: "config_sealed", PKColumn: "id",
	}
	_, err := source.UpdateSealedPayload(context.Background(), crypto.SealedPayloadWrite{
		Column: col, PK: "c1", Expected: "plaintext", Sealed: "",
	})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet(), "an empty replacement must never reach the database")
}

func TestUpdateSealedPayload_ErrorIsWrapped(t *testing.T) {
	mock, source := newSourceDB(t)
	col := crypto.SealedPayloadColumn{
		Table: "guliangiam.pay_channel_configs", Column: "config_sealed", PKColumn: "id",
	}
	cause := errors.New("database unavailable")
	mock.ExpectExec("UPDATE guliangiam.pay_channel_configs SET config_sealed = $1 WHERE id = $2 AND config_sealed = $3").
		WithArgs("sealed-token", "c1", "plaintext").
		WillReturnError(cause)

	_, err := source.UpdateSealedPayload(context.Background(), crypto.SealedPayloadWrite{
		Column: col, PK: "c1", Expected: "plaintext", Sealed: "sealed-token",
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInternal, appErr.Code)
	assert.ErrorIs(t, err, cause)
	require.NoError(t, mock.ExpectationsWereMet())
}
