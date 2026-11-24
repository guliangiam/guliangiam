package uow

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	pgtenant "github.com/guliangiam/guliangiam/internal/infra/postgres"
	platformdb "github.com/guliangiam/guliangiam/internal/platform/db"
)

// TestTenantFilterCrossScopesToSingleTenant is the real-PostgreSQL evidence for
// OPEN-API-42 C1: after ScopeAuthorizer allows an explicit target tenant, the
// request runs as role=member with a single non-empty organization, so it sees
// exactly that tenant's rows and cannot write into another tenant.
//
// Two DSNs are required because the fixture must bypass FORCE RLS while the
// assertion must be subject to it:
//
//	POSTGRES_DSN             migration/owner role (superuser or BYPASSRLS)
//	GIAM_TEST_PG_RLS_DSN     NOSUPERUSER, NOBYPASSRLS role with DML on guliangiam
//
// Run:
//
//	POSTGRES_DSN='postgres://owner@/disposable?host=/tmp&sslmode=disable' \
//	GIAM_TEST_PG_RLS_DSN='postgres://rls_user:...@127.0.0.1:5432/disposable?sslmode=disable' \
//	  go test -run TestTenantFilterCrossScopesToSingleTenant ./internal/infra/postgres/uow/
func TestTenantFilterCrossScopesToSingleTenant(t *testing.T) {
	fixtureDSN := os.Getenv("POSTGRES_DSN")
	if fixtureDSN == "" {
		fixtureDSN = os.Getenv("DATABASE_URL")
	}
	rlsDSN := os.Getenv("GIAM_TEST_PG_RLS_DSN")
	if fixtureDSN == "" || rlsDSN == "" {
		t.Skip("POSTGRES_DSN and GIAM_TEST_PG_RLS_DSN are required for the cross-tenant RLS test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fixture, err := pgxpool.New(ctx, fixtureDSN)
	require.NoError(t, err)
	t.Cleanup(fixture.Close)

	suffix := uuid.NewString()[:8]
	orgA, orgB := uuid.New(), uuid.New()
	userA, userB := uuid.New(), uuid.New()
	withSuperAdmin := func(ctx context.Context) (pgx.Tx, error) {
		tx, err := fixture.Begin(ctx)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, "SELECT set_config('app.current_role', 'super_admin', true)"); err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
		return tx, nil
	}

	tx, err := withSuperAdmin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1, $2, $3), ($4, $5, $6)",
		orgA, "cross-a-"+suffix, "Cross A", orgB, "cross-b-"+suffix, "Cross B")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.users (id, organization_id, username) VALUES ($1, $2, $3), ($4, $5, $6)",
		userA, orgA, "user-a-"+suffix, userB, orgB, "user-b-"+suffix)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		cleanupTx, beginErr := withSuperAdmin(cleanupCtx)
		if beginErr != nil {
			return
		}
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.users WHERE id = ANY($1::uuid[])", []uuid.UUID{userA, userB})
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.organizations WHERE id = ANY($1::uuid[])", []uuid.UUID{orgA, orgB})
		_ = cleanupTx.Commit(cleanupCtx)
	})

	// The application pool runs as a non-superuser role, so PrepareConn's
	// member+target GUC is what PostgreSQL enforces.
	pool, err := platformdb.NewPostgresPool(ctx, platformdb.PostgresConfig{
		DataSourceName: rlsDSN,
		MaxConns:       2,
	})
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	crossCtx := pgtenant.WithTenantScope(context.Background(), orgA.String(), "", pgtenant.TenantFilterCross)

	var orgCount int
	require.NoError(t, pool.QueryRow(crossCtx, "SELECT count(*) FROM guliangiam.organizations").Scan(&orgCount))
	require.Equal(t, 1, orgCount, "cross scope must see exactly the target organization")

	var userCount int
	require.NoError(t, pool.QueryRow(crossCtx, "SELECT count(*) FROM guliangiam.users").Scan(&userCount))
	require.Equal(t, 1, userCount, "cross scope must see only the target tenant's users")

	var visibleUser string
	require.NoError(t, pool.QueryRow(crossCtx, "SELECT id::text FROM guliangiam.users").Scan(&visibleUser))
	require.Equal(t, userA.String(), visibleUser)

	// Writing into another tenant must be rejected by the RLS WITH CHECK.
	_, err = pool.Exec(crossCtx,
		"INSERT INTO guliangiam.users (id, organization_id, username) VALUES ($1, $2, $3)",
		uuid.New(), orgB, "cross-write-"+suffix)
	require.Error(t, err, "cross scope must not insert into another tenant")

	// Updating another tenant's row must affect zero rows.
	tag, err := pool.Exec(crossCtx, "UPDATE guliangiam.users SET username = $1 WHERE id = $2", "cross-updated-"+suffix, userB)
	require.NoError(t, err)
	require.Zero(t, tag.RowsAffected())
}
