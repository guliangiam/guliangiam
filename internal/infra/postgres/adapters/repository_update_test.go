package adapters

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	adapter := &domain.Adapter{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, adapter)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	adapter := &domain.Adapter{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, adapter)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	adapter := &domain.Adapter{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, adapter)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), adapter.Revision)
}

// TestUpdate_OrganizationPredicate pins that the entity's OrganizationID is
// carried as the last WHERE argument so an update can never move rows across
// tenants.
func TestUpdate_OrganizationPredicate(t *testing.T) {
	ctx := context.Background()
	adapter := &domain.Adapter{ID: "adapter-1", OrganizationID: "org-1", Revision: 2}
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL, gotArgs = sql, args
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	require.NoError(t, repo.Update(ctx, adapter))

	assert.Contains(t, gotSQL, "AND organization_id=$22::uuid")
	require.Len(t, gotArgs, 22)
	assert.Equal(t, "adapter-1", gotArgs[20])
	assert.Equal(t, int64(1), gotArgs[19], "revision argument is the pre-increment revision")
	assert.Equal(t, "org-1", gotArgs[21])
}
