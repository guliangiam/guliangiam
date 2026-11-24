package notificationproviders

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
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

func TestRepository_List_DecryptsAccessToken(t *testing.T) {
	sealed, err := crypto.GlobalFieldEncryptor().EncryptField("token-abc")
	require.NoError(t, err)

	mockDB := &shared.MockDBTX{
		QueryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return shared.NewMockRows([][]any{{
				"np-001", "org-001", "ses", "email", "https://hooks.example.com",
				sealed, []byte(`{}`), true, nil, nil, int64(1),
			}}), nil
		},
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(1)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	items, total, err := repo.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "token-abc", items[0].AccessToken)
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
