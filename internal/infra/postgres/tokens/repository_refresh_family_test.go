package tokens

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-D-2：CreateRefresh 必须把 PostgreSQL 约束错误映射为平台错误
// （家族自引用 FK 23503 → 422，唯一哈希 23505 → 409），而不是裸 500。
func TestRepository_CreateRefresh_MapsConstraintErrors(t *testing.T) {
	cases := []struct {
		name       string
		pgCode     string
		constraint string
		wantCode   apperrors.Code
		wantStatus int
	}{
		{"family FK violation is 422", "23503", "fk_refresh_tokens_family_id_refresh_tokens", apperrors.CodeUnprocessableEntity, 422},
		{"unique hash violation is 409", "23505", "uq_refresh_tokens_hash", apperrors.CodeConflict, 409},
		{"invalid uuid text is 422", "22P02", "", apperrors.CodeUnprocessableEntity, 422},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := &shared.MockDBTX{
				ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					return pgconn.CommandTag{}, &pgconn.PgError{
						Code:           tc.pgCode,
						Message:        "driver detail that must not reach the client",
						ConstraintName: tc.constraint,
						TableName:      "refresh_tokens",
					}
				},
			}
			repo := NewRepository(mockDB)
			err := repo.CreateRefresh(context.Background(), &domain.RefreshToken{ID: "rt-1", FamilyID: "rt-1"})

			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr, "CreateRefresh must map the PostgreSQL error")
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HTTPStatus())
			assert.NotContains(t, appErr.Message, "driver detail")
		})
	}
}

// BE-D-3：family_id 可空列必须以 COALESCE 兜底扫描，历史 NULL 行不得让
// GetRefreshByTokenHash 直接 500。
func TestQueryGetRefreshTokenByHash_CoalescesNullableFamilyID(t *testing.T) {
	assert.True(t,
		strings.Contains(queryGetRefreshTokenByHash, "COALESCE(t.family_id::text, '')"),
		"refresh token lookup must coalesce nullable family_id for the string scan")
}
