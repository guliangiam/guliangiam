package metering

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// 回归：分页 total 必须用独立计数 SQL。原实现把 `SELECT count(*)` 拼在
// `SELECT id, ...` 前，会生成 `SELECT count(*) SELECT id, ...`（42601）。
func TestListEvents_CountSQLIsStandalone(t *testing.T) {
	var countSQL string
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			countSQL = sql
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
		QueryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
			return shared.NewMockRows(nil), nil
		},
	}

	_, _, err := NewRepository(db).ListEvents(context.Background(), domain.ListEventsQuery{
		OrganizationID: "org-1",
		Page:           1,
		PageSize:       20,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(strings.TrimSpace(countSQL), "SELECT count(*) FROM"),
		"count SQL must be standalone: %s", countSQL)
	require.NotContains(t, countSQL, "SELECT id,")
}

func TestListAggregates_CountSQLIsStandalone(t *testing.T) {
	var countSQL string
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			countSQL = sql
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
		QueryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
			return shared.NewMockRows(nil), nil
		},
	}

	_, _, err := NewRepository(db).ListAggregates(context.Background(), domain.ListAggregatesQuery{
		OrganizationID: "org-1",
		Page:           1,
		PageSize:       20,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(strings.TrimSpace(countSQL), "SELECT count(*) FROM"),
		"count SQL must be standalone: %s", countSQL)
	require.NotContains(t, countSQL, "SELECT id,")
}

// TestRefreshAggregate_GeneratesUUIDID pins that the upsert gets a valid UUID
// primary key, while the conflict key (org, period, subject, metric) stays the
// business key. Repeating the refresh keeps the business args identical and
// only rotates the candidate primary key.
func TestRefreshAggregate_GeneratesUUIDID(t *testing.T) {
	var (
		generated []string
		argSets   [][]any
	)
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			identity, _ := args[4].(string)
			generated = append(generated, identity)
			argSets = append(argSets, append([]any(nil), args...))
			return shared.NewMockRow([]any{
				identity, args[0], args[3], args[1], args[2],
				int64(42), "count", nil, nil,
			}, nil)
		},
	}

	repo := NewRepository(db)
	cmd := domain.RefreshAggregateCommand{
		OrganizationID: "00000000-0000-0000-0000-000000000001",
		Subject:        "user-123",
		Metric:         "login",
		Period:         "2026-09",
	}
	for i := 0; i < 2; i++ {
		agg, err := repo.RefreshAggregate(context.Background(), cmd)
		require.NoError(t, err)
		require.NotNil(t, agg)
		require.Equal(t, int64(42), agg.Quantity)
	}

	require.Len(t, generated, 2)
	require.True(t, platformuuid.IsValid(generated[0]), "generated id must be a UUID: %s", generated[0])
	require.True(t, platformuuid.IsValid(generated[1]), "generated id must be a UUID: %s", generated[1])
	require.NotEqual(t, generated[0], generated[1])

	// args: 0 org, 1 subject, 2 metric, 3 period, 4 id, 5..6 time bounds.
	require.Equal(t, argSets[0][0], argSets[1][0])
	require.Equal(t, argSets[0][1], argSets[1][1])
	require.Equal(t, argSets[0][2], argSets[1][2])
	require.Equal(t, argSets[0][3], argSets[1][3])
	require.NotEqual(t, argSets[0][4], argSets[1][4])
}
