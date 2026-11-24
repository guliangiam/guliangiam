package captchaproviders

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

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

// TestRepository_GetByID_TenantPredicate pins that the tenant is part of the
// lookup itself: a by-ID read must never rely on RLS alone.
func TestRepository_GetByID_TenantPredicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL, gotArgs = sql, args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByID(context.Background(), "test-id", "org-1")
	require.Error(t, err)

	assert.Equal(t, []any{"test-id", "org-1"}, gotArgs)
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)",
		"by-ID lookup must carry the organization predicate")
}
