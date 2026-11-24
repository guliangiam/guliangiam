package webauthn

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestUpdateSignCount_ExecError(t *testing.T) {
	ctx := context.Background()
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateSignCount(ctx, "cred-1", int64(5))
	assert.Error(t, err)
}

func TestUpdateSignCount_Success(t *testing.T) {
	ctx := context.Background()
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateSignCount(ctx, "cred-1", int64(5))
	assert.NoError(t, err)
}
