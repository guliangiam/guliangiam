package roles

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_Create_MapsConstraintErrors(t *testing.T) {
	cases := []struct {
		name       string
		pgCode     string
		constraint string
		wantCode   apperrors.Code
		wantStatus int
	}{
		{"duplicate name is 409", "23505", "uq_roles_org_name", apperrors.CodeConflict, 409},
		{"unknown organization is 422", "23503", "fk_roles_organization_id_organizations", apperrors.CodeUnprocessableEntity, 422},
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
						TableName:      "roles",
					}
				},
			}
			err := NewRepository(mockDB).Create(context.Background(), &domain.Role{})

			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr, "repository must map the PostgreSQL error to a platform error")
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HTTPStatus())
			assert.NotContains(t, appErr.Message, "driver detail")
		})
	}
}
