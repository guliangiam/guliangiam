package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_Count_WithOrgID(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(42)}, nil)
		},
	}
	repo := NewRepository(db)
	count, err := repo.Count(context.Background(), "org1")
	assert.NoError(t, err)
	assert.Equal(t, int64(42), count)
}

func TestRepository_Count_WithoutOrgID(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(100)}, nil)
		},
	}
	repo := NewRepository(db)
	count, err := repo.Count(context.Background(), "")
	assert.NoError(t, err)
	assert.Equal(t, int64(100), count)
}

func TestRepository_Count_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.Count(context.Background(), "org1")
	assert.Error(t, err)
}

func TestRepository_HasDeletedWithUsername_True(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{true}, nil)
		},
	}
	repo := NewRepository(db)
	exists, err := repo.HasDeletedWithUsername(context.Background(), "org1", "deleteduser")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestRepository_HasDeletedWithUsername_False(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{false}, nil)
		},
	}
	repo := NewRepository(db)
	exists, err := repo.HasDeletedWithUsername(context.Background(), "org1", "activeuser")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestRepository_HasDeletedWithUsername_Error(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.HasDeletedWithUsername(context.Background(), "org1", "user1")
	assert.Error(t, err)
}

func TestRepository_ListRoleBindingsByUserIDs_Empty(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	result, err := repo.ListRoleBindingsByUserIDs(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Equal(t, map[string][]domain.UserRoleBinding{}, result)
}

func TestRepository_ListRoleBindingsByUserIDs_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewRepository(db)
	_, err := repo.ListRoleBindingsByUserIDs(context.Background(), []string{"user1"})
	assert.Error(t, err)
}

func TestRepository_ListRoleBindingsByUserIDs_Success(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return shared.NewMockRows([][]any{
				{"user1", "role1"},
				{"user1", "role2"},
				{"user2", "role1"},
			}), nil
		},
	}
	repo := NewRepository(db)
	result, err := repo.ListRoleBindingsByUserIDs(context.Background(), []string{"user1", "user2"})
	assert.NoError(t, err)
	assert.Len(t, result["user1"], 2)
	assert.Len(t, result["user2"], 1)
}

func TestRepository_ListGroupMembershipsByUserIDs_Empty(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	result, err := repo.ListGroupMembershipsByUserIDs(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Equal(t, map[string][]domain.UserGroupMembership{}, result)
}

func TestRepository_ListGroupMembershipsByUserIDs_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewRepository(db)
	_, err := repo.ListGroupMembershipsByUserIDs(context.Background(), []string{"user1"})
	assert.Error(t, err)
}

func TestRepository_ListGroupMembershipsByUserIDs_Success(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return shared.NewMockRows([][]any{
				{"user1", "group1"},
				{"user2", "group2"},
			}), nil
		},
	}
	repo := NewRepository(db)
	result, err := repo.ListGroupMembershipsByUserIDs(context.Background(), []string{"user1", "user2"})
	assert.NoError(t, err)
	assert.Len(t, result["user1"], 1)
	assert.Len(t, result["user2"], 1)
}

func TestRepository_ReplaceIdentities_Success(t *testing.T) {
	callCount := 0
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			callCount++
			return pgconn.NewCommandTag("DELETE"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.ReplaceIdentities(context.Background(), "user1", []domain.UserIdentity{
		{ID: "id1", UserID: "user1", ProviderID: "prov1", IdentityType: "external", Subject: "sub1", ProfileSnapshot: map[string]interface{}{}, LinkedAt: time.Now()},
	})
	assert.NoError(t, err)
	assert.True(t, callCount >= 2) // DELETE + at least one INSERT
}

func TestRepository_ReplaceIdentities_MarshalError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.ReplaceIdentities(context.Background(), "user1", []domain.UserIdentity{
		{ID: "id1", UserID: "user1", ProviderID: "prov1", IdentityType: "external", Subject: "sub1", ProfileSnapshot: map[string]any{"bad": func() {}}, LinkedAt: time.Now()},
	})
	assert.Error(t, err)
}

func TestRepository_CreateIdentity_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.CreateIdentity(context.Background(), &domain.UserIdentity{ID: "id1", UserID: "user1", ProviderID: "prov1", ProfileSnapshot: map[string]interface{}{}})
	assert.NoError(t, err)
}

func TestRepository_DeleteIdentity_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.DeleteIdentity(context.Background(), "user1", "id1")
	assert.NoError(t, err)
}

func TestRepository_ReplaceMfaMethods_Success(t *testing.T) {
	callCount := 0
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			callCount++
			return pgconn.NewCommandTag("DELETE"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.ReplaceMfaMethods(context.Background(), "user1", []domain.UserMfaMethod{
		{ID: "m1", UserID: "user1", MethodType: "totp", Enabled: true, Verified: true},
	})
	assert.NoError(t, err)
	assert.True(t, callCount >= 2)
}

func TestRepository_ReplaceRoleBindings_Success(t *testing.T) {
	callCount := 0
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			callCount++
			return pgconn.NewCommandTag("DELETE"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.ReplaceRoleBindings(context.Background(), "user1", []domain.UserRoleBinding{
		{UserID: "user1", RoleID: "role1"},
	})
	assert.NoError(t, err)
	assert.True(t, callCount >= 2)
}

func TestRepository_ReplaceGroupMemberships_Success(t *testing.T) {
	callCount := 0
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			callCount++
			return pgconn.NewCommandTag("DELETE"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.ReplaceGroupMemberships(context.Background(), "user1", []domain.UserGroupMembership{
		{UserID: "user1", GroupID: "group1"},
	})
	assert.NoError(t, err)
	assert.True(t, callCount >= 2)
}

func TestRepository_GetPasswordHistory_Success(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return shared.NewMockRows([][]any{}), nil
		},
	}
	repo := NewRepository(db)
	entries, err := repo.GetPasswordHistory(context.Background(), "user1", 5)
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

func TestRepository_CreatePasswordHistory_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.CreatePasswordHistory(context.Background(), &domain.PasswordHistoryEntry{ID: "ph1", UserID: "user1"})
	assert.NoError(t, err)
}

func TestRepository_DeletePasswordHistoryBeyond_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.DeletePasswordHistoryBeyond(context.Background(), "user1", 5)
	assert.NoError(t, err)
}

func TestRepository_WithinTransaction_NonBeginner(t *testing.T) {
	// When db doesn't implement beginner, withinTransaction calls fn directly
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.withinTransaction(context.Background(), func(ctx context.Context, tx shared.DBTX) error {
		_, err := tx.Exec(ctx, "SELECT 1")
		return err
	})
	assert.NoError(t, err)
}

func TestRepository_WithinTransaction_NonBeginner_Error(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	err := repo.withinTransaction(context.Background(), func(ctx context.Context, tx shared.DBTX) error {
		return errors.New("fn error")
	})
	assert.Error(t, err)
}

func TestRepository_batchLoadProfiles_Empty(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	err := repo.batchLoadProfiles(context.Background(), []*domain.User{})
	assert.NoError(t, err)
}

func TestRepository_batchLoadCredentials_Empty(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	err := repo.batchLoadCredentials(context.Background(), []*domain.User{})
	assert.NoError(t, err)
}

func TestRepository_batchLoadIdentities_Empty(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	err := repo.batchLoadIdentities(context.Background(), []*domain.User{})
	assert.NoError(t, err)
}

func TestRepository_batchLoadMfaMethods_Empty(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	err := repo.batchLoadMfaMethods(context.Background(), []*domain.User{})
	assert.NoError(t, err)
}

func TestRepository_batchLoadProfiles_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewRepository(db)
	err := repo.batchLoadProfiles(context.Background(), []*domain.User{{ID: "user1"}})
	assert.Error(t, err)
}

func TestRepository_batchLoadCredentials_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewRepository(db)
	err := repo.batchLoadCredentials(context.Background(), []*domain.User{{ID: "user1"}})
	assert.Error(t, err)
}

func TestRepository_batchLoadIdentities_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewRepository(db)
	err := repo.batchLoadIdentities(context.Background(), []*domain.User{{ID: "user1"}})
	assert.Error(t, err)
}

func TestRepository_batchLoadMfaMethods_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewRepository(db)
	err := repo.batchLoadMfaMethods(context.Background(), []*domain.User{{ID: "user1"}})
	assert.Error(t, err)
}
