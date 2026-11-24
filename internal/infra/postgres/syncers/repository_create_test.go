package syncers

import (
	"context"
	"testing"

	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_Create_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Syncer{}
	err := repo.Create(context.Background(), entity)
	require.NoError(t, err)
}

func TestRepository_Create_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Syncer{}
	err := repo.Create(context.Background(), entity)
	require.Error(t, err)
}
