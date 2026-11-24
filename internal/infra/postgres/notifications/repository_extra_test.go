package notifications

import (
	"context"
	"fmt"
	"testing"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestRepository_List_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("query error")
		},
	}
	repo := NewRepository(mockDB)
	_, _, err := repo.List(context.Background(), "org-1", "", "", 1, 10)
	assert.Error(t, err)
}

func TestRepository_List_KeywordReachesQuery(t *testing.T) {
	var listArgs []any
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			listArgs = args
			return nil, fmt.Errorf("stop after capture")
		},
	}
	repo := NewRepository(mockDB)
	_, _, _ = repo.List(context.Background(), "org-1", "pending", "reset", 1, 10)
	if assert.Len(t, listArgs, 5) {
		assert.Equal(t, "org-1", listArgs[0])
		assert.Equal(t, "pending", listArgs[1])
		assert.Equal(t, "reset", listArgs[2])
	}
}

func TestRepository_Delete_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Delete(context.Background(), "notif-1")
	assert.Error(t, err)
}

func TestRepository_Delete_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Delete(context.Background(), "notif-1")
	assert.NoError(t, err)
}

func TestRepository_MarkSent_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.MarkSent(context.Background(), "notif-1")
	assert.Error(t, err)
}

func TestRepository_MarkSent_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.MarkSent(context.Background(), "notif-1")
	assert.NoError(t, err)
}

func TestRepository_MarkFailed_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.MarkFailed(context.Background(), "notif-1", "delivery failed")
	assert.Error(t, err)
}

func TestRepository_MarkFailed_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.MarkFailed(context.Background(), "notif-1", "delivery failed")
	assert.NoError(t, err)
}

func TestRepository_IncrementRetryCount_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.IncrementRetryCount(context.Background(), "notif-1")
	assert.Error(t, err)
}

func TestRepository_IncrementRetryCount_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.IncrementRetryCount(context.Background(), "notif-1")
	assert.NoError(t, err)
}

func TestRepository_ListPendingRetries_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("query error")
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.ListPendingRetries(context.Background(), 10)
	assert.Error(t, err)
}

func TestNotificationsMax(t *testing.T) {
	assert.Equal(t, 5, maxInt(5, 0))
	assert.Equal(t, 10, maxInt(0, 10))
	assert.Equal(t, 3, maxInt(-1, 3))
}

func TestNullable(t *testing.T) {
	s := nullable("hello")
	assert.NotNil(t, s)
	assert.Equal(t, "hello", *s)

	s2 := nullable("")
	assert.Nil(t, s2)
}
