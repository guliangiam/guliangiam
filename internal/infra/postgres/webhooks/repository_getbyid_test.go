package webhooks

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestRepository_GetByID_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByID(context.Background(), "test-id")
	require.Error(t, err)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByID(context.Background(), "test-id")
	require.Error(t, err)
}

// TestRepository_GetByIDForOrganization_TenantPredicate pins that the by-ID
// management read binds organization_id in SQL; a foreign row is ErrNoRows.
func TestRepository_GetByIDForOrganization_TenantPredicate(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($2)")
			captured = append([]any(nil), args...)
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	_, err := NewRepository(db).GetByIDForOrganization(context.Background(), "wh-1", "org-1")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	require.Len(t, captured, 2)
	assert.Equal(t, "wh-1", captured[0])
	assert.Equal(t, "org-1", captured[1])
}
