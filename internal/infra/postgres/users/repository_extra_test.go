package users

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_GetByID_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByID(context.Background(), "user1")
	assert.Error(t, err)
}

func TestRepository_GetByUsername_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByUsername(context.Background(), "org1", "user1")
	assert.Error(t, err)
}

func TestRepository_GetByLoginIdentifier_NotFound(t *testing.T) {
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			gotArgs = args
			return shared.NewMockRows(nil), nil
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByLoginIdentifier(context.Background(), "org1", "alice@example.com")
	assert.Error(t, err)
	assert.Equal(t, []any{"org1", "alice@example.com"}, gotArgs)
}

func TestRepository_GetByLoginIdentifier_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByLoginIdentifier(context.Background(), "org1", "alice")
	assert.Error(t, err)
}

func TestRepository_GetByLoginIdentifier_BlankIdentifierSkipsQuery(t *testing.T) {
	queried := false
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			queried = true
			return shared.NewMockRows(nil), nil
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByLoginIdentifier(context.Background(), "org1", "   ")
	assert.Error(t, err)
	assert.False(t, queried)
}

func TestRepository_GetByProviderIdentity_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByProviderIdentity(context.Background(), "prov1", "sub1")
	assert.Error(t, err)
}

func TestRepository_ReplaceIdentities_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.ReplaceIdentities(context.Background(), "user1", []domain.UserIdentity{})
	assert.Error(t, err)
}

func TestRepository_CreateIdentity_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.CreateIdentity(context.Background(), &domain.UserIdentity{ID: "id1", UserID: "user1"})
	assert.Error(t, err)
}

func TestRepository_DeleteIdentity_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.DeleteIdentity(context.Background(), "user1", "id1")
	assert.Error(t, err)
}

func TestRepository_ReplaceMfaMethods_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.ReplaceMfaMethods(context.Background(), "user1", []domain.UserMfaMethod{})
	assert.Error(t, err)
}

func TestRepository_ReplaceRoleBindings_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.ReplaceRoleBindings(context.Background(), "user1", []domain.UserRoleBinding{})
	assert.Error(t, err)
}

func TestRepository_ReplaceGroupMemberships_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.ReplaceGroupMemberships(context.Background(), "user1", []domain.UserGroupMembership{})
	assert.Error(t, err)
}

func TestRepository_GetPasswordHistory_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetPasswordHistory(context.Background(), "user1", 5)
	assert.Error(t, err)
}

func TestRepository_CreatePasswordHistory_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.CreatePasswordHistory(context.Background(), &domain.PasswordHistoryEntry{ID: "ph1", UserID: "user1"})
	assert.Error(t, err)
}

func TestRepository_DeletePasswordHistoryBeyond_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.DeletePasswordHistoryBeyond(context.Background(), "user1", 5)
	assert.Error(t, err)
}
