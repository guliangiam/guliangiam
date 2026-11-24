package payments

import (
	"context"
	"testing"

	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_Create_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Payment{}
	err := repo.Create(context.Background(), entity)
	require.NoError(t, err)
}

func TestChannelConfigsRepository_CreateIncludesProviderID(t *testing.T) {
	var capturedArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			require.Equal(t, QueryChannelConfigInsert, sql)
			capturedArgs = args
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewChannelConfigsRepository(mockDB)
	config := &domain.PayChannelConfig{ID: "detail-1", OrganizationID: "org-1", ProviderID: "provider-1"}
	err := repo.Create(context.Background(), config)
	require.NoError(t, err)
	require.Len(t, capturedArgs, 17)
	require.Equal(t, "provider-1", capturedArgs[2])
}

func TestRepository_Create_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Payment{}
	err := repo.Create(context.Background(), entity)
	require.Error(t, err)
}

func TestRepository_Update_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Payment{}
	err := repo.Update(context.Background(), entity)
	require.NoError(t, err)
}

func TestRepository_Update_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Payment{}
	err := repo.Update(context.Background(), entity)
	require.Error(t, err)
}
