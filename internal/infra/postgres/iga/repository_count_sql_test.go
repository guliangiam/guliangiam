package iga

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// 回归：列表页的分页 total 使用独立计数 SQL。把 "SELECT count(*) " 直接拼在
// `SELECT id, ...` 前面会产生 `SELECT count(*) SELECT id, ...`，
// PostgreSQL 报 42601（syntax error at or near "id"），/iga 页面 500。
func TestListCampaigns_CountSQLIsStandalone(t *testing.T) {
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

	_, _, err := NewRepository(db).ListCampaigns(context.Background(), domain.ListCampaignsQuery{
		OrganizationID: "org-1",
		Page:           1,
		PageSize:       20,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(strings.TrimSpace(countSQL), "SELECT count(*) FROM"),
		"count SQL must be standalone: %s", countSQL)
	require.NotContains(t, countSQL, "SELECT id,")
}

func TestListLifecycle_CountSQLIsStandalone(t *testing.T) {
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

	_, _, err := NewRepository(db).ListLifecycle(context.Background(), domain.ListLifecycleQuery{
		OrganizationID: "org-1",
		Page:           1,
		PageSize:       20,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(strings.TrimSpace(countSQL), "SELECT count(*) FROM"),
		"count SQL must be standalone: %s", countSQL)
	require.NotContains(t, countSQL, "SELECT id,")
}
