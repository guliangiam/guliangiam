package verifications

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestUpdateStatus_ExecError(t *testing.T) {
	ctx := context.Background()
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateStatus(ctx, "id-1", "verified")
	assert.Error(t, err)
}

func TestUpdateStatus_Success(t *testing.T) {
	ctx := context.Background()
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateStatus(ctx, "id-1", "verified")
	assert.NoError(t, err)
}
