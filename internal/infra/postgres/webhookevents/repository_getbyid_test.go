package webhookevents

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

// TestRepository_GetByIDForOrganization_ParentTenantPredicate pins that the
// event read is bound to the parent webhook's organization_id; webhook_events
// has no organization_id column.
func TestRepository_GetByIDForOrganization_ParentTenantPredicate(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, `EXISTS (SELECT 1 FROM "guliangiam".webhooks w`)
			assert.Contains(t, sql, "w.organization_id = guliangiam.resolve_org_id($2)")
			captured = append([]any(nil), args...)
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	_, err := NewRepository(db).GetByIDForOrganization(context.Background(), "evt-1", "org-1")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	require.Len(t, captured, 2)
	assert.Equal(t, "evt-1", captured[0])
	assert.Equal(t, "org-1", captured[1])
}
