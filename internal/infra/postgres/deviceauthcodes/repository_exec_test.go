package deviceauthcodes

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceauthcodes_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestDeviceauthcodes_Create(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.DeviceAuthCode{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "INSERT")
}

func TestDeviceauthcodes_Create_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.DeviceAuthCode{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	assert.Error(t, err)
}

func TestDeviceauthcodes_UpdateVerification(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.DeviceAuthCode{ID: "test-1"}
	err := repo.UpdateVerification(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "UPDATE")
}

func TestDeviceauthcodes_UpdateVerification_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.DeviceAuthCode{ID: "test-1"}
	err := repo.UpdateVerification(t.Context(), item)
	assert.Error(t, err)
}

func TestDeviceauthcodes_DeleteByDeviceCode(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteByDeviceCode(t.Context(), "test-1")
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "DELETE")
}

func TestDeviceauthcodes_DeleteByDeviceCode_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteByDeviceCode(t.Context(), "test-1")
	assert.Error(t, err)
}

func TestDeviceauthcodes_DeleteByUserCode(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteByUserCode(t.Context(), "test-1")
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "DELETE")
}

func TestDeviceauthcodes_DeleteByUserCode_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteByUserCode(t.Context(), "test-1")
	assert.Error(t, err)
}

func TestDeviceauthcodes_DeleteExpired(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	_, err := repo.DeleteExpired(t.Context())
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "DELETE")
}

func TestDeviceauthcodes_DeleteExpired_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	_, err := repo.DeleteExpired(t.Context())
	assert.Error(t, err)
}
