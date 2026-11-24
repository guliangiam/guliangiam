package keys

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
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

func TestRepository_ListDoesNotReadPrivateKey(t *testing.T) {
	var listSQL string
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			listSQL = sql
			return shared.NewMockRows([][]any{{
				"key-1", "org-1", "name", "Name", "RSA", "public-key", nil,
				"RS256", nil, true, nil, nil, int64(1),
			}}), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(1)}, nil)
		},
	}

	items, total, err := NewRepository(mockDB).List(context.Background(), domain.ListQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Empty(t, items[0].PrivateKey)
	assert.Contains(t, listSQL, "NULL::text AS private_key")
	assert.NotContains(t, strings.SplitN(listSQL, " FROM ", 2)[0], "keys.private_key")
}
