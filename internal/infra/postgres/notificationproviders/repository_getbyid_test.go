package notificationproviders

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func TestRepository_GetByID_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByID(context.Background(), "test-id")
	require.Error(t, err)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByID(context.Background(), "test-id")
	require.Error(t, err)
}

func TestRepository_GetByID_DecryptsAccessToken(t *testing.T) {
	sealed, err := crypto.GlobalFieldEncryptor().EncryptField("token-abc")
	require.NoError(t, err)

	mockDB := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow([]any{
				"np-001", "org-001", "ses", "email", "https://hooks.example.com",
				sealed, []byte(`{"region":"us-east-1"}`), true, nil, nil, int64(1),
			}, nil)
		},
	}
	repo := NewRepository(mockDB)
	provider, err := repo.GetByID(context.Background(), "np-001")
	require.NoError(t, err)
	require.Equal(t, "token-abc", provider.AccessToken)
}

func TestRepository_GetByID_PlaintextFailsClosed(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow([]any{
				"np-001", "org-001", "ses", "email", "https://hooks.example.com",
				"legacy-plaintext", []byte(`{}`), true, nil, nil, int64(1),
			}, nil)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByID(context.Background(), "np-001")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not envelope-encrypted")
	require.NotContains(t, err.Error(), "legacy-plaintext")
}
