package agents

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/agents"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	agent := &domain.Agent{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, agent)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	agent := &domain.Agent{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, agent)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	agent := &domain.Agent{ID: "agent-1", OrganizationID: "org-1", Revision: 2}
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow([]any{int64(3)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, agent)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), agent.Revision)

	require.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($12)",
		"by-ID updates must be scoped by the entity organization")
	require.Len(t, gotArgs, 12)
	assert.Equal(t, "agent-1", gotArgs[10])
	assert.Equal(t, int64(1), gotArgs[9], "optimistic guard uses the pre-increment revision")
	assert.Equal(t, "org-1", gotArgs[11])
}
