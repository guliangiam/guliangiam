package mcpservers

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestRepository_GetByIDForOrganization_BindsOrganizationPredicate(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			capturedSQL = sql
			capturedArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)

	_, err := repo.GetByIDForOrganization(context.Background(), "server-1", "org-1")

	requireNotFound(t, err)
	assert.Equal(t, QueryGetByIDForOrganization, capturedSQL)
	assert.Contains(t, capturedSQL, "organization_id")
	assert.Equal(t, []any{"server-1", "org-1"}, capturedArgs)
}

func TestRepository_Update_OrganizationPredicateAndZeroRows(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := NewRepository(mockDB)

	err := repo.Update(context.Background(), &domain.McpServer{ID: "server-1", OrganizationID: "org-1"})

	requireNotFound(t, err)
	assert.Contains(t, capturedSQL, "organization_id")
	require.Len(t, capturedArgs, 10)
	assert.Equal(t, "server-1", capturedArgs[8])
	assert.Equal(t, "org-1", capturedArgs[9])
}

func TestRepository_Delete_OrganizationPredicateAndZeroRows(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	repo := NewRepository(mockDB)

	err := repo.Delete(context.Background(), "server-1", "org-1")

	requireNotFound(t, err)
	assert.Contains(t, capturedSQL, "organization_id")
	assert.Equal(t, []any{"server-1", "org-1"}, capturedArgs)
}
