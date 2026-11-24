package applications

import (
	"context"
	"testing"

	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestRepository_Create_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Application{}
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
	entity := &domain.Application{}
	err := repo.Create(context.Background(), entity)
	require.Error(t, err)
}

// 并发同名创建时预检会被两个请求同时通过，落败方在 INSERT 上命中
// uq_applications_org_slug (23505)。该错误必须在仓库边界收敛为 409，
// 否则会冒泡成 500（写成功、界面报“服务暂时不可用”）。
func TestRepository_Create_MapsUniqueViolationToAlreadyExists(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, &pgconn.PgError{
				Code:           pgerror.CodeUniqueViolation,
				ConstraintName: "uq_applications_org_slug",
			}
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Create(context.Background(), &domain.Application{})
	require.Error(t, err)

	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.CodeAlreadyExists, appErr.Code)
}
