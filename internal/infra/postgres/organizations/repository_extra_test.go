package organizations

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/stretchr/testify/assert"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_GetByID_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByID(context.Background(), "org1")
	assert.Error(t, err)
}

func TestRepository_GetBySlug_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetBySlug(context.Background(), "my-org")
	assert.Error(t, err)
}

func TestRepository_HasDeletedWithSlug_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.HasDeletedWithSlug(context.Background(), "my-org")
	assert.Error(t, err)
}

func TestRepository_HasDeletedWithSlug_False(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{false}, nil)
		},
	}
	repo := NewRepository(db)
	result, err := repo.HasDeletedWithSlug(context.Background(), "my-org")
	assert.NoError(t, err)
	assert.False(t, result)
}

func TestRepository_HasDeletedWithSlug_True(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{true}, nil)
		},
	}
	repo := NewRepository(db)
	result, err := repo.HasDeletedWithSlug(context.Background(), "my-org")
	assert.NoError(t, err)
	assert.True(t, result)
}

func TestRepository_LoadAggregate_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	// getAggregateBy calls QueryRow then loadAggregate
	_, err := repo.GetByID(context.Background(), "org1")
	assert.Error(t, err)
}
