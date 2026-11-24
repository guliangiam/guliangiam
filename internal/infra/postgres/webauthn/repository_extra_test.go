package webauthn

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_GetByCredentialID_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByCredentialID(context.Background(), []byte("cred1"))
	assert.Error(t, err)
}

func TestRepository_ListByUserID_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewRepository(db)
	_, err := repo.ListByUserID(context.Background(), "user1")
	assert.Error(t, err)
}
