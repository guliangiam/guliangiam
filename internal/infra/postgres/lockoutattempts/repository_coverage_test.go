package lockoutattempts

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockoutAttempts_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestLockoutAttempts_Create(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db)

	now := time.Now()
	attempt := &domain.Attempt{
		ID:          "lock-1",
		UserID:      "user-1",
		Attempts:    3,
		LockedUntil: &now,
		Revision:    1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err := repo.Create(t.Context(), attempt)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "INSERT")
	assert.Contains(t, capturedArgs, "lock-1")
	assert.Contains(t, capturedArgs, "user-1")
	assert.Equal(t, int64(1), attempt.Revision)
}

func TestLockoutAttempts_Update(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)

	now := time.Now()
	attempt := &domain.Attempt{
		ID:          "lock-1",
		UserID:      "user-1",
		Attempts:    5,
		LockedUntil: &now,
		Revision:    1,
		UpdatedAt:   now,
	}

	err := repo.Update(t.Context(), attempt)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "UPDATE")
	assert.Equal(t, int64(2), attempt.Revision)
}

func TestLockoutAttempts_Update_RevisionConflict(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := NewRepository(db)
	attempt := &domain.Attempt{ID: "lock-1", UserID: "user-1", Revision: 1}

	err := repo.Update(t.Context(), attempt)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeConflict, err.(*apperrors.Error).Code)
	assert.Equal(t, int64(1), attempt.Revision)
}

func TestLockoutAttempts_DeleteByUserID(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteByUserID(t.Context(), "user-1")
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "DELETE")
	assert.Contains(t, capturedArgs, "user-1")
}

func TestLockoutAttempts_DeleteExpired(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("UPDATE 5"), nil
		},
	}
	repo := NewRepository(db)

	affected, err := repo.DeleteExpired(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(5), affected)
	assert.Contains(t, capturedSQL, "UPDATE")
	assert.Contains(t, capturedSQL, "attempts = 0")
	assert.NotContains(t, capturedSQL, "lockout_level =")
}

func TestLockoutAttempts_DeleteExpired_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	_, err := repo.DeleteExpired(t.Context())
	assert.Error(t, err)
}

func TestLockoutAttempts_Create_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	attempt := &domain.Attempt{ID: "lock-1", UserID: "user-1", Revision: 1}
	err := repo.Create(t.Context(), attempt)
	assert.Error(t, err)
}

func TestLockoutAttempts_Update_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	attempt := &domain.Attempt{ID: "lock-1", UserID: "user-1"}
	err := repo.Update(t.Context(), attempt)
	assert.Error(t, err)
}

func TestLockoutAttempts_DeleteByUserID_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteByUserID(t.Context(), "user-1")
	assert.Error(t, err)
}
