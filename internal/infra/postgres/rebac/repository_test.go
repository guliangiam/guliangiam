package rebac

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ─── NewRepository ─────────────────────────────────────────────────────────

func TestRebac_NewRepository(t *testing.T) {
	t.Parallel()
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestRebac_ImplementsDomainInterface(t *testing.T) {
	t.Parallel()
	var _ domain.Repository = Repository{}
}

// ─── GetNamespace (error paths with pgx.ErrNoRows) ────────────────────────

func TestRebac_GetNamespace_NotFound(t *testing.T) {
	t.Parallel()
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetNamespace(context.Background(), "missing-id", "org-1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.Equal(t, 404, appErr.HTTPStatus())
}

func TestRebac_GetNamespaceByName_NotFound(t *testing.T) {
	t.Parallel()
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetNamespaceByName(context.Background(), "org-1", "missing")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestRebac_GetNamespace_DBError(t *testing.T) {
	t.Parallel()
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("connection reset"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetNamespace(context.Background(), "ns-1", "org-1")
	require.Error(t, err)
	assert.EqualError(t, err, "connection reset")
}

// ─── DeleteNamespace ────────────────────────────────────────────────────────

func TestRebac_DeleteNamespace_Success(t *testing.T) {
	t.Parallel()
	var calledSQL string
	var calledArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			calledSQL = sql
			calledArgs = args
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.DeleteNamespace(context.Background(), "ns-1", "org-1")
	require.NoError(t, err)

	assert.Contains(t, calledSQL, "DELETE FROM guliangiam.relation_namespaces")
	assert.Equal(t, []any{"ns-1", "org-1"}, calledArgs)
}

func TestRebac_DeleteNamespace_DriverError(t *testing.T) {
	t.Parallel()
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("unique_violation")
		},
	}
	repo := NewRepository(db)
	err := repo.DeleteNamespace(context.Background(), "ns-1", "org-1")
	require.Error(t, err)
	assert.EqualError(t, err, "unique_violation")
}

// ─── DeleteTuple ────────────────────────────────────────────────────────────

func TestRebac_DeleteTuple_Success(t *testing.T) {
	t.Parallel()
	var calledSQL string
	var calledArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			calledSQL = sql
			calledArgs = args
			return shared.NewMockRow([]any{"ns-1"}, nil)
		},
	}
	repo := NewRepository(db)
	err := repo.DeleteTuple(context.Background(), "tpl-1", "org-1")
	require.NoError(t, err)
	assert.Equal(t, []any{"tpl-1", "org-1"}, calledArgs)
	// 删除必须与写入对称地 bump namespace revision（zookie 单调推进）。
	assert.Contains(t, calledSQL, "DELETE FROM guliangiam.relation_tuples")
	assert.Contains(t, calledSQL, "revision_counter = revision_counter + 1")
}

func TestRebac_DeleteTuple_NotFound(t *testing.T) {
	t.Parallel()
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)
	err := repo.DeleteTuple(context.Background(), "missing", "org-1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.Equal(t, 404, appErr.HTTPStatus())
}

func TestRebac_DeleteTuple_DriverError(t *testing.T) {
	t.Parallel()
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("lock_not_available"))
		},
	}
	repo := NewRepository(db)
	err := repo.DeleteTuple(context.Background(), "tpl-1", "org-1")
	require.Error(t, err)
}

// ─── DeleteTuplesByNamespace ────────────────────────────────────────────────

func TestRebac_DeleteTuplesByNamespace_Success(t *testing.T) {
	t.Parallel()
	var calledArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			calledArgs = args
			return pgconn.NewCommandTag("DELETE 42"), nil
		},
	}
	repo := NewRepository(db)
	count, err := repo.DeleteTuplesByNamespace(context.Background(), "ns-1", "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(42), count)
	assert.Equal(t, []any{"ns-1", "org-1"}, calledArgs)
}

func TestRebac_DeleteTuplesByNamespace_ZeroRows(t *testing.T) {
	t.Parallel()
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	repo := NewRepository(db)
	count, err := repo.DeleteTuplesByNamespace(context.Background(), "ns-empty", "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestRebac_DeleteTuplesByNamespace_DriverError(t *testing.T) {
	t.Parallel()
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("deadlock detected")
		},
	}
	repo := NewRepository(db)
	count, err := repo.DeleteTuplesByNamespace(context.Background(), "ns-1", "org-1")
	require.Error(t, err)
	assert.Equal(t, int64(0), count)
}
