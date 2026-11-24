package forms

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/forms"
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
		wantCode   apperrors.Code
		wantStatus int
	}{
		{"unique violation is 409", "23505", apperrors.CodeConflict, 409},
		{"check violation is 422", "23514", apperrors.CodeUnprocessableEntity, 422},
		{"invalid text representation is 422", "22P02", apperrors.CodeUnprocessableEntity, 422},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := &shared.MockDBTX{
				ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					return pgconn.CommandTag{}, &pgconn.PgError{
						Code:           tc.pgCode,
						Message:        "driver detail that must not reach the client",
						ConstraintName: "uq_forms_org_name",
						TableName:      "forms",
					}
				},
			}
			repo := NewRepository(mockDB)
			err := repo.Create(context.Background(), &domain.Form{})

			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr, "repository must map the PostgreSQL error to a platform error")
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HTTPStatus())
			assert.NotContains(t, appErr.Message, "driver detail")
		})
	}
}
