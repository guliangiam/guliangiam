package castickets

import (
	"context"
	"fmt"
	"testing"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestRepository_GetByTicketID_QueryRowError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByTicketID(context.Background(), "ticket-1")
	assert.Error(t, err)
}

func TestRepository_GetByTicketID_NotFoundV2(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByTicketID(context.Background(), "ticket-1")
	assert.Error(t, err)
}

func TestRepository_DeleteByTicketID_QueryRowError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.DeleteByTicketID(context.Background(), "ticket-1")
	assert.Error(t, err)
}

func TestRepository_DeleteByTicketID_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.DeleteByTicketID(context.Background(), "ticket-1")
	assert.Error(t, err)
}

func TestRepository_DeleteExpired_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.DeleteExpired(context.Background())
	assert.Error(t, err)
}

func TestRepository_DeleteExpired_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 5"), nil
		},
	}
	repo := NewRepository(mockDB)
	count, err := repo.DeleteExpired(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)
}
