package aigateway

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUsageLogsRepository_QueryStats_AggregatesAllSections(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(queryStatsTotals + queryStatsWhereOrg)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests", "succeeded", "failed", "input_tokens", "output_tokens",
			"total_tokens", "cost_usd", "avg_ttft", "avg_latency",
		}).AddRow(int64(3), int64(2), int64(1), int64(100), int64(200), int64(300), 1.25, 120.5, 800.0))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByProvider)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"provider_id", "requests", "tokens", "cost_usd"}).
			AddRow("prov-1", int64(2), int64(200), 0.75))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByDay)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"day", "tokens", "cost_usd"}).
			AddRow(day, int64(200), 0.75))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByProtocol)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"protocol_face", "requests", "tokens"}).
			AddRow("openai", int64(2), int64(200)))

	repo := NewUsageLogsRepository(db)
	stats, err := repo.QueryStats(context.Background(), "org-1", 0)
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.TotalRequests)
	require.Equal(t, int64(300), stats.TotalTokens)
	require.InDelta(t, 1.25, stats.TotalCostUSD, 0.0001)
	require.NotNil(t, stats.AvgTTFTMs)
	require.InDelta(t, 120.5, *stats.AvgTTFTMs, 0.0001)
	require.NotNil(t, stats.AvgLatencyMs)
	require.InDelta(t, 800.0, *stats.AvgLatencyMs, 0.0001)
	require.Len(t, stats.ByProvider, 1)
	require.Equal(t, "prov-1", stats.ByProvider[0].ProviderID)
	require.Len(t, stats.ByDay, 1)
	require.Equal(t, "2026-01-02", stats.ByDay[0].Date)
	require.Len(t, stats.ByProtocol, 1)
	require.Equal(t, "openai", stats.ByProtocol[0].ProtocolFace)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogsRepository_QueryStats_PropagatesProviderRowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	wantErr := errors.New("driver: connection reset")

	mock.ExpectQuery(regexp.QuoteMeta(queryStatsTotals + queryStatsWhereOrg)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests", "succeeded", "failed", "input_tokens", "output_tokens",
			"total_tokens", "cost_usd", "avg_ttft", "avg_latency",
		}).AddRow(int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), 0.0, nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByProvider)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"provider_id", "requests", "tokens", "cost_usd"}).
			AddRow("prov-1", int64(1), int64(10), 0.0).
			RowError(0, wantErr))

	repo := NewUsageLogsRepository(db)
	_, err = repo.QueryStats(context.Background(), "org-1", 0)
	require.ErrorIs(t, err, wantErr,
		"partial aggregate must not be returned when the provider cursor fails")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogsRepository_QueryStats_PropagatesDayRowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	wantErr := errors.New("driver: day cursor failed")

	mock.ExpectQuery(regexp.QuoteMeta(queryStatsTotals + queryStatsWhereOrg)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests", "succeeded", "failed", "input_tokens", "output_tokens",
			"total_tokens", "cost_usd", "avg_ttft", "avg_latency",
		}).AddRow(int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), 0.0, nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByProvider)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"provider_id", "requests", "tokens", "cost_usd"}))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByDay)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"day", "tokens", "cost_usd"}).
			AddRow(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), int64(10), 0.0).
			RowError(0, wantErr))

	repo := NewUsageLogsRepository(db)
	_, err = repo.QueryStats(context.Background(), "org-1", 0)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogsRepository_QueryStats_PropagatesProtocolRowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	wantErr := errors.New("driver: protocol cursor failed")

	mock.ExpectQuery(regexp.QuoteMeta(queryStatsTotals + queryStatsWhereOrg)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests", "succeeded", "failed", "input_tokens", "output_tokens",
			"total_tokens", "cost_usd", "avg_ttft", "avg_latency",
		}).AddRow(int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), 0.0, nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByProvider)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"provider_id", "requests", "tokens", "cost_usd"}))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByDay)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"day", "tokens", "cost_usd"}))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByProtocol)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"protocol_face", "requests", "tokens"}).
			AddRow("openai", int64(1), int64(10)).
			RowError(0, wantErr))

	repo := NewUsageLogsRepository(db)
	_, err = repo.QueryStats(context.Background(), "org-1", 0)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogsRepository_QueryStats_DaysWindowUsesDateSub(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryStatsTotals+queryStatsWhereOrgDays)).
		WithArgs("org-1", 7).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests", "succeeded", "failed", "input_tokens", "output_tokens",
			"total_tokens", "cost_usd", "avg_ttft", "avg_latency",
		}).AddRow(int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), 0.0, nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByProviderDays)).
		WithArgs("org-1", 7).
		WillReturnRows(sqlmock.NewRows([]string{"provider_id", "requests", "tokens", "cost_usd"}))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByDayDays)).
		WithArgs("org-1", 7).
		WillReturnRows(sqlmock.NewRows([]string{"day", "tokens", "cost_usd"}))
	mock.ExpectQuery(regexp.QuoteMeta(queryStatsByProtocolDays)).
		WithArgs("org-1", 7).
		WillReturnRows(sqlmock.NewRows([]string{"protocol_face", "requests", "tokens"}))

	repo := NewUsageLogsRepository(db)
	stats, err := repo.QueryStats(context.Background(), "org-1", 7)
	require.NoError(t, err)
	require.Equal(t, int64(0), stats.TotalRequests)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogsRepository_QueryStats_EmptyOrgReturnsEmptyStats(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewUsageLogsRepository(db)
	stats, err := repo.QueryStats(context.Background(), "", 7)
	require.NoError(t, err)
	require.Equal(t, int64(0), stats.TotalRequests)
	require.Empty(t, stats.ByProvider)
}
