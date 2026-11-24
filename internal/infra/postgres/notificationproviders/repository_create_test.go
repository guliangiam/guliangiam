package notificationproviders

import (
	"context"
	"testing"

	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func TestRepository_Create_Success(t *testing.T) {
	var capturedArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			require.Equal(t, queryInsertNotificationProvider, sql)
			capturedArgs = args
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.NotificationProvider{ID: "provider-1"}
	err := repo.Create(context.Background(), entity)
	require.NoError(t, err)
	require.Len(t, capturedArgs, 12)
	// provider_id 回落到 NULL（模型未提供该关联）。
	require.Nil(t, capturedArgs[2])
}

func TestRepository_Create_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.NotificationProvider{}
	err := repo.Create(context.Background(), entity)
	require.Error(t, err)
}

func TestRepository_Create_EncryptsAccessToken(t *testing.T) {
	const plaintext = "access-token-plaintext"
	provider := &domain.NotificationProvider{ID: "provider-secret", AccessToken: plaintext}
	var captured []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			require.Equal(t, queryInsertNotificationProvider, sql)
			captured = args
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	require.NoError(t, repo.Create(context.Background(), provider))

	sealed, ok := captured[6].(string)
	require.True(t, ok)
	require.NotEqual(t, plaintext, sealed)
	require.True(t, crypto.IsEnvelopeToken(sealed))
	opened, err := crypto.GlobalFieldEncryptor().DecryptEnvelopeFieldWithAAD(notificationProviderAAD(provider.ID), sealed)
	require.NoError(t, err)
	require.Equal(t, plaintext, opened)
	require.Equal(t, plaintext, provider.AccessToken, "sealing must not mutate the domain object")
}
