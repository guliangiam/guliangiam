package syncers

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_GetByIDForOrganization_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByIDForOrganization(context.Background(), "test-id", "org-1")
	require.Error(t, err)
}

func TestRepository_GetByIDForOrganization_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByIDForOrganization(context.Background(), "test-id", "org-1")
	require.Error(t, err)
}

func TestRepository_GetByIDForOrganization_PassesTrustedOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByIDForOrganization(context.Background(), "test-id", "org-1")
	require.Error(t, err)
	require.Contains(t, gotSQL, "organization_id", "the by-ID read must carry the tenant predicate")
	require.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Len(t, gotArgs, 2)
	require.Equal(t, "test-id", gotArgs[0])
	require.Equal(t, "org-1", gotArgs[1])
}
