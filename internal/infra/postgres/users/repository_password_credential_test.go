package users

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// 定向更新口必须只写口令列；任何 access_* 列出现在 SQL 里都意味着
// 自助改密会顺带清空访问密钥。无凭据行的用户（外部身份/无口令创建）
// 必须经 upsert 落库，否则管理员重置口令会静默空写（200 但不持久化）。
func TestUsers_UpdatePasswordCredential_WritesOnlyPasswordColumns(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		capturedSQL = sql
		capturedArgs = args
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}

	updatedAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	repo := NewRepository(db)
	err := repo.UpdatePasswordCredential(context.Background(), &userdomain.UserCredentials{
		UserID:             "user-1",
		PasswordHash:       "argon2id-hash",
		PasswordAlgo:       userdomain.PasswordAlgorithmArgon2ID,
		PasswordUpdatedAt:  &updatedAt,
		NeedUpdatePassword: true,
	})
	require.NoError(t, err)

	assert.Contains(t, capturedSQL, "password_hash")
	assert.Contains(t, capturedSQL, "password_algo")
	assert.Contains(t, capturedSQL, "password_updated_at")
	assert.Contains(t, capturedSQL, "need_update_password")
	assert.NotContains(t, capturedSQL, "access_key")
	assert.NotContains(t, capturedSQL, "access_secret_hash")
	assert.NotContains(t, capturedSQL, "access_secret_rotated_at")
	assert.Contains(t, capturedSQL, "INSERT INTO guliangiam.user_credentials")
	assert.Contains(t, capturedSQL, "ON CONFLICT (user_id) DO UPDATE")

	require.Len(t, capturedArgs, 5)
	assert.Equal(t, "user-1", capturedArgs[0])
	assert.Equal(t, "argon2id-hash", capturedArgs[1])
	assert.Equal(t, userdomain.PasswordAlgorithmArgon2ID, capturedArgs[2])
	assert.Equal(t, &updatedAt, capturedArgs[3])
	assert.Equal(t, true, capturedArgs[4])
}
