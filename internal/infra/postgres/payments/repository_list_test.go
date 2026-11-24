package payments

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
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

func TestCallbackReceiptsRepository_ListUsesCountQuery(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			require.Equal(t, ReceiptCount, sql)
			require.Equal(t, []any{"org-1"}, args)
			return shared.NewMockRow([]any{int64(3)}, nil)
		},
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			require.Equal(t, ReceiptList, sql)
			require.Equal(t, []any{"org-1", 20, 40}, args)
			return shared.NewMockRows(nil), nil
		},
	}
	repo := NewCallbackReceiptsRepository(mockDB)
	items, total, err := repo.List(t.Context(), "org-1", 20, 40)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Equal(t, int64(3), total)
}
