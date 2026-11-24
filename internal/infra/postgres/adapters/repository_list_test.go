package adapters

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
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

// The two tests above keep `args` opaque, so nothing in this package asserted
// that a filter reaches the statement. That gap is why `databaseType` could be
// declared on `ListQuery`, set by the handler, and never sent: the row scan
// succeeded and the page came back unfiltered.
//
// The argument *count* is held by `scripts/check-sql-args.py` (which sees the
// call site, unlike PREPARE); this test pins the *position and value*, which is
// what the count cannot express.
func TestRepository_List_ForwardsFilters(t *testing.T) {
	var listArgs []any
	mockDB := &shared.MockDBTX{
		QueryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
			listArgs = args
			return shared.NewMockRows(nil), nil
		},
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
	}

	enabled := true
	_, _, err := NewRepository(mockDB).List(context.Background(), domain.ListQuery{
		Page: 2, PageSize: 10,
		OrganizationID: "org-1", Keyword: "kw",
		IsEnabled: &enabled, DatabaseType: "mysql",
	})
	require.NoError(t, err)

	// Statement order: $1 organization, $2 keyword, $3 is_enabled,
	// $4 database_type, $5 limit, $6 offset.
	require.Len(t, listArgs, 6)
	require.Equal(t, "org-1", listArgs[0])
	require.Equal(t, "kw", listArgs[1])
	require.Equal(t, &enabled, listArgs[2])
	require.Equal(t, "mysql", listArgs[3])
	require.Equal(t, 10, listArgs[4])
	require.Equal(t, 10, listArgs[5])
}
