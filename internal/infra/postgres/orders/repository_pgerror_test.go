package orders

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 运行期复验发现：PlaceOrder 传入非 UUID 的 user 时，user_id::uuid 触发 22P02，
// 旧实现直接返回原始 *pgconn.PgError 泄漏成 500。Create/Update 必须经 pgerror.Map
// 映射为 4xx，且不得把驱动细节透传给客户端。
func TestRepository_Create_MapsPgErrors(t *testing.T) {
	cases := []struct {
		name       string
		pgCode     string
		constraint string
		wantCode   apperrors.Code
		wantStatus int
	}{
		{"invalid uuid text is 422", "22P02", "", apperrors.CodeUnprocessableEntity, 422},
		{"duplicate org name is 409", "23505", "uq_orders_org_name", apperrors.CodeConflict, 409},
		{"unknown organization is 422", "23503", "fk_orders_organization_id_organizations", apperrors.CodeUnprocessableEntity, 422},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := &shared.MockDBTX{
				ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					return pgconn.CommandTag{}, &pgconn.PgError{
						Code:           tc.pgCode,
						Message:        "driver detail that must not reach the client",
						ConstraintName: tc.constraint,
						TableName:      "orders",
					}
				},
			}
			err := NewRepository(mockDB).Create(context.Background(), &domain.Order{})
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HTTPStatus())
			assert.NotContains(t, appErr.Message, "driver detail")
		})
	}
}
