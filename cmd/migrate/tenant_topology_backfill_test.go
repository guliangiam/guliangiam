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

func newTopologyBackfillDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB, mock
}

func expectTopologyAggregate(mock sqlmock.Sqlmock, affected, missingIdentity, missingTopology int64) {
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(topologyBackfillRLSBypass)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(topologyBackfillAggregateSQL)).WillReturnRows(
		sqlmock.NewRows([]string{"affected_orgs", "missing_identities", "missing_topologies"}).
			AddRow(affected, missingIdentity, missingTopology))
	mock.ExpectCommit()
}

func expectTopologyPrimaryRegion(mock sqlmock.Sqlmock, regionID, regionCode, country string) {
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(topologyBackfillRLSBypass)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(topologyBackfillPrimarySQL)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "region_code", "country_code"}).AddRow(regionID, regionCode, country))
	mock.ExpectCommit()
}

func expectTopologyScan(mock sqlmock.Sqlmock, cursor string, batchSize int, rows *sqlmock.Rows) {
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(topologyBackfillRLSBypass)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(topologyBackfillScanSQL)).WithArgs(cursor, batchSize).WillReturnRows(rows)
}

func topologyScanRows(values ...driver.Value) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "slug", "missing_identity", "missing_topology"})
	for i := 0; i+3 < len(values); i += 4 {
		rows.AddRow(values[i], values[i+1], values[i+2], values[i+3])
	}
	return rows
}

func TestTenantTopologyBackfill_DryRunCountsWithoutWriting(t *testing.T) {
	sqlDB, mock := newTopologyBackfillDB(t)
	expectTopologyAggregate(mock, 3, 2, 2)

	stats, code := backfillTenantTopologyWithDB(context.Background(), sqlDB, topologyBackfillOptions{
		BatchSize: defaultTopologyBatch, DryRun: true,
	})
	assert.Equal(t, 0, code, "dry-run is advisory and always succeeds")
	assert.Equal(t, int64(3), stats.AffectedOrgs)
	assert.Equal(t, int64(2), stats.MissingIdentities)
	assert.Equal(t, int64(2), stats.MissingTopologies)
	assert.Equal(t, int64(0), stats.IdentityCreated)
	assert.Equal(t, int64(0), stats.TopologyCreated)
	require.NoError(t, mock.ExpectationsWereMet(), "dry-run must issue no INSERT")
}

func TestTenantTopologyBackfill_VerifyPassAndFail(t *testing.T) {
	sqlDB, mock := newTopologyBackfillDB(t)
	expectTopologyAggregate(mock, 0, 0, 0)
	_, code := backfillTenantTopologyWithDB(context.Background(), sqlDB, topologyBackfillOptions{Verify: true})
	assert.Equal(t, 0, code)
	require.NoError(t, mock.ExpectationsWereMet())

	sqlDB2, mock2 := newTopologyBackfillDB(t)
	expectTopologyAggregate(mock2, 1, 0, 1)
	stats, code := backfillTenantTopologyWithDB(context.Background(), sqlDB2, topologyBackfillOptions{Verify: true})
	assert.Equal(t, 1, code, "verify fails while any organization is still missing topology")
	assert.Equal(t, int64(1), stats.AffectedOrgs)
	assert.Equal(t, int64(1), stats.MissingTopologies)
	require.NoError(t, mock2.ExpectationsWereMet())
}

func TestTenantTopologyBackfill_WriteCreatesIdentityAndTopology(t *testing.T) {
	const (
		regionID   = "11111111-1111-1111-1111-111111111111"
		regionCode = "cn-north"
		country    = "CN"
		orgMount   = "22222222-2222-2222-2222-222222222222"
		orgBuiltIn = "33333333-3333-3333-3333-333333333333"
	)

	sqlDB, mock := newTopologyBackfillDB(t)
	expectTopologyPrimaryRegion(mock, regionID, regionCode, country)

	expectTopologyScan(mock, topologyBackfillZeroUUID, 2, topologyScanRows(
		orgMount, "mount", true, true,
		orgBuiltIn, "built-in", true, false,
	))
	mock.ExpectExec(regexp.QuoteMeta(topologyBackfillIdentitySQL)).WithArgs(orgMount, false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(topologyBackfillTopologySQL)).
		WithArgs(sqlmock.AnyArg(), orgMount, regionID, regionCode, country).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(topologyBackfillIdentitySQL)).WithArgs(orgBuiltIn, true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	expectTopologyScan(mock, orgBuiltIn, 2, topologyScanRows())
	mock.ExpectCommit()

	expectTopologyAggregate(mock, 0, 0, 0)

	stats, code := backfillTenantTopologyWithDB(context.Background(), sqlDB, topologyBackfillOptions{BatchSize: 2})
	assert.Equal(t, 0, code)
	assert.Equal(t, int64(2), stats.Scanned)
	assert.Equal(t, int64(2), stats.IdentityCreated)
	assert.Equal(t, int64(1), stats.TopologyCreated)
	assert.Equal(t, 1, stats.Batches)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantTopologyBackfill_SecondRunIsNoop(t *testing.T) {
	sqlDB, mock := newTopologyBackfillDB(t)
	expectTopologyPrimaryRegion(mock, "11111111-1111-1111-1111-111111111111", "cn-north", "CN")
	expectTopologyScan(mock, topologyBackfillZeroUUID, defaultTopologyBatch, topologyScanRows())
	mock.ExpectCommit()
	expectTopologyAggregate(mock, 0, 0, 0)

	stats, code := backfillTenantTopologyWithDB(context.Background(), sqlDB, topologyBackfillOptions{BatchSize: defaultTopologyBatch})
	assert.Equal(t, 0, code)
	assert.Equal(t, int64(0), stats.Scanned, "second run finds nothing to repair")
	assert.Equal(t, int64(0), stats.IdentityCreated)
	assert.Equal(t, int64(0), stats.TopologyCreated)
	require.NoError(t, mock.ExpectationsWereMet(), "no INSERT may be issued on a clean database")
}

func TestTenantTopologyBackfill_StopsWhenBatchCannotProgress(t *testing.T) {
	sqlDB, mock := newTopologyBackfillDB(t)
	expectTopologyPrimaryRegion(mock, "11111111-1111-1111-1111-111111111111", "cn-north", "CN")
	expectTopologyScan(mock, topologyBackfillZeroUUID, 2, topologyScanRows(
		"22222222-2222-2222-2222-222222222222", "mount", true, false,
	))
	mock.ExpectExec(regexp.QuoteMeta(topologyBackfillIdentitySQL)).
		WithArgs("22222222-2222-2222-2222-222222222222", false).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	stats, code := backfillTenantTopologyWithDB(context.Background(), sqlDB, topologyBackfillOptions{BatchSize: 2})
	assert.Equal(t, 1, code, "a batch that repairs nothing must fail instead of looping forever")
	assert.Equal(t, int64(1), stats.Scanned)
	assert.Equal(t, int64(0), stats.IdentityCreated)
	assert.Equal(t, int64(1), stats.IdentitySkipped)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantTopologyBackfill_ReportsRemainingRowsAfterWrite(t *testing.T) {
	sqlDB, mock := newTopologyBackfillDB(t)
	expectTopologyPrimaryRegion(mock, "11111111-1111-1111-1111-111111111111", "cn-north", "CN")
	expectTopologyScan(mock, topologyBackfillZeroUUID, 2, topologyScanRows(
		"22222222-2222-2222-2222-222222222222", "mount", true, true,
	))
	mock.ExpectExec(regexp.QuoteMeta(topologyBackfillIdentitySQL)).
		WithArgs("22222222-2222-2222-2222-222222222222", false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(topologyBackfillTopologySQL)).
		WithArgs(sqlmock.AnyArg(), "22222222-2222-2222-2222-222222222222",
			"11111111-1111-1111-1111-111111111111", "cn-north", "CN").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectTopologyScan(mock, "22222222-2222-2222-2222-222222222222", 2, topologyScanRows())
	mock.ExpectCommit()
	expectTopologyAggregate(mock, 1, 1, 1)

	stats, code := backfillTenantTopologyWithDB(context.Background(), sqlDB, topologyBackfillOptions{BatchSize: 2})
	assert.Equal(t, 1, code, "leftovers after the write pass must fail the run")
	assert.Equal(t, int64(1), stats.IdentityCreated)
	assert.Equal(t, int64(1), stats.TopologyCreated)
	assert.Equal(t, int64(1), stats.AffectedOrgs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillTenantTopology_RejectsInvalidFlags(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 2, runBackfillTenantTopology([]string{"-dry-run", "-verify"}))
	assert.Equal(t, 2, runBackfillTenantTopology([]string{"-batch-size", "0"}))
}
