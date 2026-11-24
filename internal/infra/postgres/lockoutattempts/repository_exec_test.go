package lockoutattempts

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockoutattempts_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestLockoutattempts_Create(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.Attempt{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "INSERT")
}

func TestLockoutattempts_Create_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.Attempt{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	assert.Error(t, err)
}

func TestLockoutattempts_Update(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.Attempt{ID: "test-1"}
	err := repo.Update(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "UPDATE")
}

func TestLockoutattempts_Update_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.Attempt{ID: "test-1"}
	err := repo.Update(t.Context(), item)
	assert.Error(t, err)
}

func TestLockoutattempts_DeleteByUserID(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteByUserID(t.Context(), "test-1")
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "DELETE")
}

func TestLockoutattempts_DeleteByUserID_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteByUserID(t.Context(), "test-1")
	assert.Error(t, err)
}
