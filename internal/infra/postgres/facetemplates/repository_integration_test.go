// Real-PostgreSQL regression for the revoke PREPARE type-deduction defect.
//
// queryRevokeByUser binds $3 to both revoked_at (timestamp without time zone)
// and updated_at (timestamp with time zone). Under pgx's default prepared-
// statement mode the parameter is sent untyped, so PostgreSQL must deduce a
// single type for $3 and fails with "inconsistent types deduced for parameter
// $3" — surfacing as a 500 on both the admin and self-service FaceID revoke
// routes. The mock-based unit tests never prepare real SQL and cannot catch it.
//
// This test prepares the actual statement against a migrated database. With
// random (non-existent) tenant/user UUIDs the UPDATE matches zero rows, so a
// correctly-prepared statement returns NotFound; the broken statement returns
// an internal PrepareError instead. No rows are written.
//
// Skipped unless GIAM_TEST_PG_DSN points at an already-migrated disposable
// database (same convention as the faceidproviders integration test):
//
//	GIAM_TEST_PG_DSN='postgres://.../guliangiam_verify_r1?sslmode=disable' \
//	  go test ./internal/infra/postgres/facetemplates \
//	  -run TestRevokeByUserPreparesAgainstPostgres -count=1 -v
package facetemplates

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestRevokeByUserPreparesAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL revoke prepare regression")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)

	repo := NewRepository(pool)
	// Random UUIDs match no row; the statement must still PREPARE cleanly and
	// report NotFound rather than failing with a PrepareError (internal/500).
	err = repo.RevokeByUser(ctx, uuid.NewString(), uuid.NewString(), time.Now())
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code,
		"revoke must PREPARE cleanly and report NotFound on zero rows, not a PrepareError")
}
