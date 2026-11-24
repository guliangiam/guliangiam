package webhookevents

import (
	"context"
	"testing"

	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_Create_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.WebhookEvent{}
	err := repo.Create(context.Background(), entity)
	require.NoError(t, err)
}

func TestRepository_Create_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.WebhookEvent{}
	err := repo.Create(context.Background(), entity)
	require.Error(t, err)
}

// CreateBatch must split into chunks of ≤ maxBatchInsertRows rows so each
// statement stays below the bind-parameter limit; placeholders restart per
// statement and the whole batch runs in the caller's transaction.
func TestRepository_CreateBatch_ChunksRows(t *testing.T) {
	old := maxBatchInsertRows
	maxBatchInsertRows = 2
	t.Cleanup(func() { maxBatchInsertRows = old })

	var queries []string
	var argCounts []int
	mockDB := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			queries = append(queries, sql)
			argCounts = append(argCounts, len(args))
			return pgconn.NewCommandTag("INSERT 2"), nil
		},
	}
	repo := NewRepository(mockDB)

	events := make([]*domain.WebhookEvent, 5)
	for i := range events {
		events[i] = &domain.WebhookEvent{ID: fmt.Sprintf("00000000-0000-0000-0000-%012d", i)}
	}
	require.NoError(t, repo.CreateBatch(context.Background(), events))

	require.Len(t, queries, 3, "5 rows with chunk size 2 → 3 statements")
	assert.Equal(t, []int{26, 26, 13}, argCounts, "13 params per row per chunk")
	assert.Contains(t, queries[1], "$1::uuid", "placeholders restart per chunk")
	assert.NotContains(t, queries[1], "$27", "chunk statements stay within their own row")
}

func TestRepository_Update_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.WebhookEvent{}
	err := repo.Update(context.Background(), entity)
	require.NoError(t, err)
}

func TestRepository_Update_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.WebhookEvent{}
	err := repo.Update(context.Background(), entity)
	require.Error(t, err)
}
