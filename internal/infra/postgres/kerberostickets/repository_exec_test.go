package kerberostickets

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKerberostickets_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestKerberostickets_Create(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.Ticket{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "INSERT")
}

func TestKerberostickets_Create_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.Ticket{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	assert.Error(t, err)
}

func TestKerberostickets_DeleteByTicketID(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteByTicketID(t.Context(), "test-1")
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "DELETE")
}

func TestKerberostickets_DeleteByTicketID_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteByTicketID(t.Context(), "test-1")
	assert.Error(t, err)
}

func TestKerberostickets_DeleteExpired(t *testing.T) {
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

func TestKerberostickets_DeleteExpired_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	_, err := repo.DeleteExpired(t.Context())
	assert.Error(t, err)
}
