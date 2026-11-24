package gateway

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestUpdateServer_QueryRowError(t *testing.T) {
	ctx := context.Background()
	server := &domain.Server{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateServer(ctx, server)
	assert.Error(t, err)
}

func TestUpdateServer_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	server := &domain.Server{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateServer(ctx, server)
	assert.Error(t, err)
}

func TestUpdateServer_Success(t *testing.T) {
	ctx := context.Background()
	server := &domain.Server{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateServer(ctx, server)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), server.Revision)
}

func TestUpdateSite_QueryRowError(t *testing.T) {
	ctx := context.Background()
	site := &domain.Site{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateSite(ctx, site)
	assert.Error(t, err)
}

func TestUpdateSite_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	site := &domain.Site{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateSite(ctx, site)
	assert.Error(t, err)
}

func TestUpdateSite_Success(t *testing.T) {
	ctx := context.Background()
	site := &domain.Site{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateSite(ctx, site)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), site.Revision)
}
