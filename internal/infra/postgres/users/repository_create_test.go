package users

import (
	"context"
	"testing"

	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_Create_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.User{}
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
	entity := &domain.User{}
	err := repo.Create(context.Background(), entity)
	require.Error(t, err)
}

// recovery_code_hashes 是 text[] NOT NULL（默认 '{}'）。新建用户没有恢复码时
// user.RecoveryCodeHashes 是 nil slice，pgx 会把它编码成 SQL NULL，显式 NULL
// 覆盖列默认值并触发 23502，导致 POST /api/v1/users 整体 500。
func TestRepository_Create_NilRecoveryCodeHashesStoredAsEmptyArray(t *testing.T) {
	var mfaArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "user_mfa_config") {
				mfaArgs = append([]any(nil), args...)
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	require.NoError(t, repo.Create(context.Background(), &domain.User{}))
	require.Len(t, mfaArgs, 10)
	require.Equal(t, []string{}, mfaArgs[2], "recovery_code_hashes must not be SQL NULL")
}

// 空白邮箱必须写成 NULL：uq_user_profiles_email_active 只排除 NULL，
// 若写入空字符串，第二个未填邮箱的用户会命中唯一索引，创建返回 500。
func TestRepository_Create_BlankProfileEmailStoredAsNull(t *testing.T) {
	var profileArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "user_profiles") {
				profileArgs = args
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.User{Profile: domain.UserProfile{Email: "", Phone: ""}}
	require.NoError(t, repo.Create(context.Background(), entity))
	require.NotNil(t, profileArgs)
	require.Nil(t, profileArgs[5], "email must be SQL NULL when blank")
	require.Nil(t, profileArgs[7], "phone must be SQL NULL when blank after trim")
}
