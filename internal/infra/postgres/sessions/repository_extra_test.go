package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_Touch_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.Touch(context.Background(), "sess1", time.Now())
	assert.Error(t, err)
}

func TestRepository_Touch_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.Touch(context.Background(), "sess1", time.Now())
	assert.NoError(t, err)
}

func TestRepository_Revoke_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.Revoke(context.Background(), "sess1", time.Now())
	assert.Error(t, err)
}

func TestRepository_Revoke_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.Revoke(context.Background(), "sess1", time.Now())
	assert.NoError(t, err)
}

func TestRepository_RevokeByUserExcept_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	_, err := repo.RevokeByUserExcept(context.Background(), "user1", "sess1", time.Now())
	assert.Error(t, err)
}

func TestRepository_RevokeByUserExcept_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 3"), nil
		},
	}
	repo := NewRepository(db)
	count, err := repo.RevokeByUserExcept(context.Background(), "user1", "sess1", time.Now())
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestRepository_RevokeByUserExcept_EmptyExcludeOmitsUUIDPredicate(t *testing.T) {
	revokedAt := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL, gotArgs = sql, args
			return pgconn.NewCommandTag("UPDATE 4"), nil
		},
	}
	repo := NewRepository(db)
	count, err := repo.RevokeByUserExcept(context.Background(), "user-1", "", revokedAt)
	require.NoError(t, err)
	assert.Equal(t, int64(4), count)
	assert.NotContains(t, gotSQL, "id <>")
	assert.Equal(t, []any{"user-1", revokedAt}, gotArgs)
}

func TestRepository_RevokeByUserExcept_NonEmptyExcludeKeepsUUIDPredicate(t *testing.T) {
	revokedAt := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL, gotArgs = sql, args
			return pgconn.NewCommandTag("UPDATE 2"), nil
		},
	}
	repo := NewRepository(db)
	count, err := repo.RevokeByUserExcept(context.Background(), "user-1", "session-keep", revokedAt)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Contains(t, gotSQL, "AND id <> $3")
	assert.Equal(t, []any{"user-1", revokedAt, "session-keep"}, gotArgs)
}

func TestRepository_RevokeByUserAndApp_EmptyExcludeOmitsUUIDPredicate(t *testing.T) {
	revokedAt := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL, gotArgs = sql, args
			return pgconn.NewCommandTag("UPDATE 5"), nil
		},
	}
	repo := NewRepository(db)
	count, err := repo.RevokeByUserAndApp(context.Background(), "user-1", "app-1", "", revokedAt)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
	assert.NotContains(t, gotSQL, "id <>")
	assert.Equal(t, []any{"user-1", "app-1", revokedAt}, gotArgs)
}

func TestRepository_RevokeByUserAndApp_NonEmptyExcludeKeepsUUIDPredicate(t *testing.T) {
	revokedAt := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL, gotArgs = sql, args
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)
	count, err := repo.RevokeByUserAndApp(context.Background(), "user-1", "app-1", "session-keep", revokedAt)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
	assert.Contains(t, gotSQL, "AND id <> $4")
	assert.Equal(t, []any{"user-1", "app-1", revokedAt, "session-keep"}, gotArgs)
}

func TestRepository_RevokeByUser_OmitsUUIDPredicate(t *testing.T) {
	revokedAt := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL, gotArgs = sql, args
			return pgconn.NewCommandTag("UPDATE 2"), nil
		},
	}
	repo := NewRepository(db)
	count, err := repo.RevokeByUser(context.Background(), "user-1", revokedAt)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.NotContains(t, gotSQL, "id <>")
	assert.Equal(t, []any{"user-1", revokedAt}, gotArgs)
}

func TestRepository_RevokeByUserAndApp_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	_, err := repo.RevokeByUserAndApp(context.Background(), "user1", "app1", "sess1", time.Now())
	assert.Error(t, err)
}

func TestRepository_CleanupExpired_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	_, err := repo.CleanupExpired(context.Background(), time.Now())
	assert.Error(t, err)
}

func TestRepository_CleanupExpired_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 5"), nil
		},
	}
	repo := NewRepository(db)
	count, err := repo.CleanupExpired(context.Background(), time.Now())
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)
}
