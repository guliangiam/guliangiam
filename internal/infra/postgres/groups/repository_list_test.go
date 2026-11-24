package groups

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_List_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	_, _, err := repo.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 10})
	require.Error(t, err)
}

func TestRepository_List_CountError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return shared.NewMockRows(nil), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("count error"))
		},
	}
	repo := NewRepository(mockDB)
	_, _, err := repo.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 10})
	require.Error(t, err)
}

func TestBuildGroupOrderBy(t *testing.T) {
	clause, err := buildGroupOrderBy("", "")
	require.NoError(t, err)
	require.Equal(t, "ORDER BY created_at DESC, id ASC", clause, "无参数保留历史默认 DESC")

	clause, err = buildGroupOrderBy("displayName", "asc")
	require.NoError(t, err)
	require.Equal(t, "ORDER BY display_name ASC, id ASC", clause)

	_, err = buildGroupOrderBy("password", "")
	require.Error(t, err, "白名单外字段必须报错而非静默回落")
}

func TestRepository_List_InjectsWhitelistedOrderBy(t *testing.T) {
	var gotSQL string
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			gotSQL = sql
			return shared.NewMockRows(nil), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	_, _, err := repo.List(context.Background(), domain.ListQuery{
		Page: 1, PageSize: 10, SortField: "name", SortOrder: "desc",
	})
	require.NoError(t, err)
	require.Contains(t, gotSQL, "ORDER BY name DESC, id ASC LIMIT $5 OFFSET $6")
}

// G7-s：type 过滤进入列表与计数两条 SQL，参数位 $4，分页顺延 $5/$6。
func TestRepository_List_TypeFilterReachesBothQueries(t *testing.T) {
	var listArgs, countArgs []any
	var listSQL string
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			listSQL = sql
			listArgs = args
			return shared.NewMockRows(nil), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			countArgs = args
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	_, _, err := repo.List(context.Background(), domain.ListQuery{
		Page: 1, PageSize: 10, OrganizationID: "org-1", Type: "department",
	})
	require.NoError(t, err)
	require.Contains(t, listSQL, "AND ($4 = '' OR type = $4)")
	require.Equal(t, []any{"org-1", "", "", "department", 10, 0}, listArgs)
	require.Equal(t, []any{"org-1", "", "", "department"}, countArgs)
}

func TestRepository_List_InvalidSortFieldRejected(t *testing.T) {
	repo := NewRepository(&shared.MockDBTX{})
	_, _, err := repo.List(context.Background(), domain.ListQuery{
		Page: 1, PageSize: 10, SortField: "password",
	})
	require.Error(t, err)
}
