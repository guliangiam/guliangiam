package tokens

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_GetByTokenHash_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByTokenHash(context.Background(), "hash1")
	assert.Error(t, err)
}

func TestRepository_GetByDeviceSecretHash_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetByDeviceSecretHash(context.Background(), "secret1")
	assert.Error(t, err)
}

func TestRepository_GetRefreshByTokenHash_QueryRowError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.GetRefreshByTokenHash(context.Background(), "hash1")
	assert.Error(t, err)
}

func TestRepository_CreateRefresh_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.CreateRefresh(context.Background(), &domain.RefreshToken{
		ID:        "rt1",
		Scopes:    []string{"openid"},
		Audience:  []string{"app1"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	assert.NoError(t, err)
}

func TestRepository_CreateRefresh_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.CreateRefresh(context.Background(), &domain.RefreshToken{ID: "rt1"})
	assert.Error(t, err)
}

func TestRepository_Revoke_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.Revoke(context.Background(), "t1", time.Now())
	assert.Error(t, err)
}

func TestRepository_Revoke_NoRowsAffected(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil // 0 rows affected
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("not found"))
		},
	}
	repo := NewRepository(db)
	err := repo.Revoke(context.Background(), "t1", time.Now())
	assert.Error(t, err)
}

func TestRepository_RevokeRefresh_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.RevokeRefresh(context.Background(), "rt1", time.Now())
	assert.NoError(t, err)
}

// Zero rows affected means the refresh token was already revoked, i.e. a
// concurrent request rotated the same pair first. Rotation must treat that as
// reuse and refuse to mint a second token pair.
func TestRepository_RevokeRefresh_AlreadyRevoked(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := NewRepository(db)
	err := repo.RevokeRefresh(context.Background(), "rt1", time.Now())
	assert.ErrorIs(t, err, domain.ErrRefreshAlreadyRevoked)
}

func TestRepository_RevokeRefresh_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	err := repo.RevokeRefresh(context.Background(), "rt1", time.Now())
	assert.Error(t, err)
}

func TestRepository_RevokeByQuery_Success(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return pgconn.NewCommandTag("UPDATE 5"), nil
		},
	}
	repo := NewRepository(db)
	revokedAt := time.Now()
	affected, err := repo.RevokeByQuery(context.Background(), domain.RevokeQuery{
		OrganizationID: "org1",
		UserID:         "user1",
		ApplicationID:  "app1",
		TokenPrefix:    "imp",
	}, revokedAt)

	assert.NoError(t, err)
	assert.Equal(t, int64(5), affected)

	// Bound positionally: $1 org, $2 user, $3 app, $4 revoked_at, $5 token_prefix.
	// The prefix predicate is what keeps an exit-impersonate from revoking the
	// user's own tokens, and the organization predicate is the tenant boundary
	// (access_tokens has no organization_id column, so it is bound through the
	// users join). Both must reach the statement, not just the Go struct.
	assert.Len(t, capturedArgs, 5)
	assert.Equal(t, "org1", capturedArgs[0])
	assert.Equal(t, "user1", capturedArgs[1])
	assert.Equal(t, "app1", capturedArgs[2])
	assert.Equal(t, revokedAt, capturedArgs[3])
	assert.Equal(t, "imp", capturedArgs[4])

	assert.Contains(t, capturedSQL, "u.organization_id = guliangiam.resolve_org_id($1)")
	assert.Contains(t, capturedSQL, "t.user_id = $2::uuid")
	assert.Contains(t, capturedSQL, "t.application_id = $3::uuid")
	assert.Contains(t, capturedSQL, "t.token_prefix = $5")
	assert.Contains(t, capturedSQL, "t.revoked_at IS NULL")
}

func TestRepository_RevokeByQuery_ExecError(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewRepository(db)
	_, err := repo.RevokeByQuery(context.Background(), domain.RevokeQuery{OrganizationID: "org1"}, time.Now())
	assert.Error(t, err)
}

func TestRepository_RevokeByFamilyID_Success(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			capturedSQL = sql
			capturedArgs = args
			return shared.NewMockRow([]any{int64(3)}, nil)
		},
	}
	repo := NewRepository(db)
	revokedAt := time.Now()
	affected, err := repo.RevokeByFamilyID(context.Background(), "family1", revokedAt)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), affected)
	// The count returned must be the refresh-token count, never the access-token
	// count: the reuse detector logs it as tokens_revoked and a larger number
	// would misreport the blast radius of a token-family revocation.
	assert.Contains(t, capturedSQL, "revoked_refresh")
	assert.Len(t, capturedArgs, 2)
	assert.Equal(t, "family1", capturedArgs[0])
	assert.Equal(t, revokedAt, capturedArgs[1])
}

func TestRepository_RevokeByFamilyID_ScanError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("exec error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.RevokeByFamilyID(context.Background(), "family1", time.Now())
	assert.Error(t, err)
}

// TestRepository_RevokeByAuthorizationCodeID_Success covers: a reused
// authorization code revokes everything the first redemption issued. The link is
// the code id stored in the access token metadata, and the paired refresh token
// follows through access_token_id.
func TestRepository_RevokeByAuthorizationCodeID_Success(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			capturedSQL = sql
			capturedArgs = args
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(db)
	revokedAt := time.Now()
	affected, err := repo.RevokeByAuthorizationCodeID(context.Background(), "code-1", revokedAt)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), affected)
	assert.Contains(t, capturedSQL, "metadata ->> 'authorizationCodeId'")
	assert.Contains(t, capturedSQL, "access_token_id IN (SELECT id FROM revoked_access)")
	assert.Len(t, capturedArgs, 2)
	assert.Equal(t, "code-1", capturedArgs[0])
	assert.Equal(t, revokedAt, capturedArgs[1])
}

func TestRepository_RevokeByAuthorizationCodeID_ScanError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("exec error"))
		},
	}
	repo := NewRepository(db)
	_, err := repo.RevokeByAuthorizationCodeID(context.Background(), "code-1", time.Now())
	assert.Error(t, err)
}

func TestNonNilStrings_Nil(t *testing.T) {
	result := nonNilStrings(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestNonNilStrings_Values(t *testing.T) {
	result := nonNilStrings([]string{"openid", "profile"})
	assert.Equal(t, []string{"openid", "profile"}, result)
}

func TestMarshalJSONObject_Nil(t *testing.T) {
	result, err := marshalJSONObject(nil)
	assert.NoError(t, err)
	assert.Equal(t, []byte("{}"), result)
}

func TestMarshalJSONObject_Values(t *testing.T) {
	result, err := marshalJSONObject(map[string]any{"key": "value"})
	assert.NoError(t, err)
	assert.Contains(t, string(result), "key")
}

func TestNullableString_TokensExtra(t *testing.T) {
	assert.Nil(t, nullableString(""))
	assert.Equal(t, "hello", nullableString("hello"))
}

func TestMax_TokensExtra(t *testing.T) {
	assert.Equal(t, 20, maxInt(0, 20))
	assert.Equal(t, 5, maxInt(5, 20))
	assert.Equal(t, 20, maxInt(-1, 20))
}
