package gateway

import (
	"context"
	"fmt"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/gateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestRepository_GetServerByID_QueryRowError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetServerByID(context.Background(), "server-1")
	assert.Error(t, err)
}

func TestRepository_GetServerByID_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetServerByID(context.Background(), "server-1")
	assert.Error(t, err)
}

func TestRepository_ListServers_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	_, _, err := repo.ListServers(context.Background(), gateway.ListServersQuery{OrganizationID: "org-1"})
	assert.Error(t, err)
}

func TestRepository_DeleteServer_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.DeleteServer(context.Background(), "server-1")
	assert.Error(t, err)
}

func TestRepository_DeleteServer_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.DeleteServer(context.Background(), "server-1")
	assert.NoError(t, err)
}

func TestRepository_GetSiteByID_QueryRowError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetSiteByID(context.Background(), "site-1")
	assert.Error(t, err)
}

func TestRepository_GetSiteByID_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetSiteByID(context.Background(), "site-1")
	assert.Error(t, err)
}

func TestRepository_ListSites_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	_, _, err := repo.ListSites(context.Background(), gateway.ListSitesQuery{OrganizationID: "org-1"})
	assert.Error(t, err)
}

func TestRepository_DeleteSite_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.DeleteSite(context.Background(), "site-1")
	assert.Error(t, err)
}

func TestRepository_DeleteSite_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.DeleteSite(context.Background(), "site-1")
	assert.NoError(t, err)
}
