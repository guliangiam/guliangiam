package certs

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
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

func TestRepository_GetByID_PassesTrustedOrganization(t *testing.T) {
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByID(context.Background(), "test-id", "org-1")
	require.Error(t, err)
	require.Len(t, gotArgs, 2)
	require.Equal(t, "test-id", gotArgs[0])
	require.Equal(t, "org-1", gotArgs[1])
}
