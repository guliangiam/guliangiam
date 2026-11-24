package keys

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// TestRepository_GetByID_TenantScopedQuery pins that the by-ID read carries
// both the key id and the caller's tenant inside the SQL predicate. On the PG
// baseline RLS is the second line of defence; the mirror has none.
func TestRepository_GetByID_TenantScopedQuery(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			capturedSQL = sql
			capturedArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	_, err := NewRepository(mockDB).GetByID(context.Background(), "test-id", "org-1")
	require.Error(t, err)
	assert.Contains(t, capturedSQL, "WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)")
	require.Len(t, capturedArgs, 2)
	assert.Equal(t, "test-id", capturedArgs[0])
	assert.Equal(t, "org-1", capturedArgs[1])
}

func TestRepository_GetByID_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByID(context.Background(), "test-id", "org-1")
	require.Error(t, err)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByID(context.Background(), "test-id", "org-1")
	require.Error(t, err)
}
