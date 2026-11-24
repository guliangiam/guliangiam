package providers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
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

// Classification columns (family/kind/variant) drive IDV provider resolution,
// so a list read that drops them silently disables real-name verification.
func TestRepository_List_MapsClassificationColumns(t *testing.T) {
	now := time.Now()
	row := []any{
		"p-1", "org-1", "aliyun-idv", "Aliyun IDV", "IDVerification", "AliyunIDV",
		"", "", "", "", "", "", "", "", "", "", "", "", []string{}, "", "", "", "",
		"", "cn-hangzhou", "", 0, "", []byte("{}"), "", "", "", "", "ak", "", "", "", "", "", "", "", true,
		now, now, int64(1), "identity", "aliyun_idv", "default",
	}
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return shared.NewMockRows([][]any{row}), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(1)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	items, total, err := repo.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, domain.Family("identity"), items[0].Family)
	require.Equal(t, domain.Kind("aliyun_idv"), items[0].Kind)
	require.Equal(t, domain.Variant("default"), items[0].Variant)
}
