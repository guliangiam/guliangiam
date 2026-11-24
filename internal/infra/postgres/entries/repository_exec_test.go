package entries

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntries_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestEntries_Create(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.Entry{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "INSERT")
}

func TestEntries_Create_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.Entry{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	assert.Error(t, err)
}

func TestEntries_Update(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.Entry{ID: "test-1"}
	err := repo.Update(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "UPDATE")
}

func TestEntries_Update_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.Entry{ID: "test-1"}
	err := repo.Update(t.Context(), item)
	assert.Error(t, err)
}

func TestEntries_Delete(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.Delete(t.Context(), "test-1", "org-1")
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "DELETE")
	assert.Contains(t, capturedSQL, "organization_id")
}

func TestEntries_Delete_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	err := repo.Delete(t.Context(), "test-1", "org-1")
	assert.Error(t, err)
}
