package providers

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_GetConfig_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetConfig(context.Background(), "prov1")
	assert.Error(t, err)
}

func TestRepository_ListSecretStatuses_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewRepository(db)
	_, err := repo.ListSecretStatuses(context.Background(), "prov1")
	assert.Error(t, err)
}
