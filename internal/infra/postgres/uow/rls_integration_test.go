package uow

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	applicationsapp "github.com/guliangiam/guliangiam/internal/app/applications"
	pgtenant "github.com/guliangiam/guliangiam/internal/infra/postgres"
	platformdb "github.com/guliangiam/guliangiam/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestSetTenantContextFailsClosedAndResetsPooledConnection(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}

	if dsn == "" {
		t.Skip("POSTGRES_DSN or DATABASE_URL is required for the RLS integration test")
	}

	ctx := context.Background()
	pool, err := platformdb.NewPostgresPool(ctx, platformdb.PostgresConfig{
		DataSourceName: dsn,
		MaxConns:       1,
	})
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var appRoleExists bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'guliangiam_app')
	`).Scan(&appRoleExists))
	require.True(t, appRoleExists, "schema bootstrap must create the non-owner application role")

	tableName := fmt.Sprintf("rls_probe_%d", time.Now().UnixNano())
	table := pgx.Identifier{"guliangiam", tableName}.Sanitize()
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			organization_id uuid NOT NULL,
			value text NOT NULL
		)
	`, table))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, dropErr := pool.Exec(ctx, "DROP TABLE "+table)
		require.NoError(t, dropErr)
	})
	_, err = pool.Exec(ctx, "ALTER TABLE "+table+" ENABLE ROW LEVEL SECURITY")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "ALTER TABLE "+table+" FORCE ROW LEVEL SECURITY")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		CREATE POLICY tenant_isolation ON %s
			FOR ALL
			USING (guliangiam.is_super_admin()
			       OR organization_id = guliangiam.current_organization_id())
			WITH CHECK (guliangiam.is_super_admin()
			            OR organization_id = guliangiam.current_organization_id())
	`, table))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "GRANT SELECT, INSERT, UPDATE, DELETE ON "+table+" TO guliangiam_app")
	require.NoError(t, err)

	const organizationA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const organizationB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	_, err = pool.Exec(ctx,
		"INSERT INTO "+table+" (organization_id, value) VALUES ($1, 'tenant-a'), ($2, 'tenant-b')",
		organizationA, organizationB,
	)
	require.NoError(t, err)

	var baselineRole, baselineOrganizationID string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT current_setting('app.current_role', true),
		       COALESCE(current_setting('app.current_organization_id', true), '')
	`).Scan(&baselineRole, &baselineOrganizationID))

	tests := []struct {
		name             string
		ctx              context.Context
		wantSuperAdmin   bool
		wantOrganization string
		rollback         bool
	}{
		{name: "missing tenant scope fails closed", ctx: ctx},
		{
			name: "tenant scope is local to transaction",
			ctx: pgtenant.WithTenantScope(
				ctx,
				"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
				"",
				pgtenant.TenantFilterOptional,
			),
			wantOrganization: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		},
		{
			name: "rollback does not leak tenant scope",
			ctx: pgtenant.WithTenantScope(
				ctx,
				"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
				"",
				pgtenant.TenantFilterOptional,
			),
			wantOrganization: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
			rollback:         true,
		},
		{
			name:           "super admin requires explicit scope",
			ctx:            pgtenant.WithTenantScope(ctx, "", "", pgtenant.TenantFilterSuperAdmin),
			wantSuperAdmin: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(ctx) //nolint:errcheck // rollback in test cleanup; error intentionally ignored // rollback in test cleanup; error intentionally ignored

			_, err = tx.Exec(ctx, "SET LOCAL ROLE guliangiam_app")
			require.NoError(t, err)
			require.NoError(t, setTenantContext(tt.ctx, tx))

			var currentUser string
			require.NoError(t, tx.QueryRow(ctx, "SELECT current_user").Scan(&currentUser))
			require.Equal(t, "guliangiam_app", currentUser, "RLS must be tested as a non-owner role")

			var roleCanBypassRLS, isTableOwner bool
			require.NoError(t, tx.QueryRow(ctx, `
				SELECT r.rolsuper OR r.rolbypassrls,
				       c.relowner = r.oid
				FROM pg_roles r
				JOIN pg_class c ON c.oid = $1::regclass
				WHERE r.rolname = current_user
			`, table).Scan(&roleCanBypassRLS, &isTableOwner))
			require.False(t, roleCanBypassRLS, "application role must not have superuser or BYPASSRLS privilege")
			require.False(t, isTableOwner, "application role must not own the protected table")

			var forceRowSecurity bool
			require.NoError(t, tx.QueryRow(ctx, `
				SELECT relforcerowsecurity
				FROM pg_class
				WHERE oid = $1::regclass
			`, table).Scan(&forceRowSecurity))
			require.True(t, forceRowSecurity, "fixture must exercise FORCE ROW LEVEL SECURITY")

			var visibleRows int
			require.NoError(t, tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&visibleRows))
			switch {
			case tt.wantSuperAdmin:
				require.Equal(t, 2, visibleRows)
			case tt.wantOrganization != "":
				require.Equal(t, 1, visibleRows)
				var visibleOrganization string
				require.NoError(t, tx.QueryRow(ctx,
					"SELECT organization_id::text FROM "+table,
				).Scan(&visibleOrganization))
				require.Equal(t, tt.wantOrganization, visibleOrganization)
			default:
				require.Zero(t, visibleRows, "missing tenant context must fail closed")
			}

			otherOrganization := organizationB
			if tt.wantOrganization == organizationB {
				otherOrganization = organizationA
			}
			tag, err := tx.Exec(ctx,
				"UPDATE "+table+" SET value = 'modified' WHERE organization_id = $1",
				otherOrganization,
			)
			require.NoError(t, err)
			if tt.wantSuperAdmin {
				require.EqualValues(t, 1, tag.RowsAffected())
			} else {
				require.Zero(t, tag.RowsAffected(), "RLS must prevent updating another tenant's row")
			}

			if tt.rollback {
				require.NoError(t, tx.Rollback(ctx))
			} else {
				require.NoError(t, tx.Commit(ctx))
			}

			var roleAfter, organizationAfter string
			require.NoError(t, pool.QueryRow(ctx, `
				SELECT current_setting('app.current_role', true),
				       COALESCE(current_setting('app.current_organization_id', true), '')
			`).Scan(&roleAfter, &organizationAfter))
			require.Equal(t, baselineRole, roleAfter, "transaction role must not leak to the pooled connection")
			require.Equal(t, baselineOrganizationID, organizationAfter, "transaction organization must not leak to the pooled connection")
		})
	}

	t.Run("write policy rejects another tenant", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx) //nolint:errcheck // rollback in test cleanup; error intentionally ignored

		_, err = tx.Exec(ctx, "SET LOCAL ROLE guliangiam_app")
		require.NoError(t, err)
		scope := pgtenant.WithTenantScope(ctx, organizationA, "", pgtenant.TenantFilterOptional)
		require.NoError(t, setTenantContext(scope, tx))

		_, err = tx.Exec(ctx,
			"INSERT INTO "+table+" (organization_id, value) VALUES ($1, 'forbidden')",
			organizationB,
		)
		require.Error(t, err, "RLS WITH CHECK must reject cross-tenant writes")
		require.NoError(t, tx.Rollback(ctx))
	})
}

func TestResolveBuiltInApplicationWithTenantScope(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("POSTGRES_DSN or DATABASE_URL is required for the RLS integration test")
	}

	ctx := pgtenant.WithTenantScope(
		context.Background(),
		"built-in",
		"built-in",
		pgtenant.TenantFilterOptional,
	)
	pool, err := platformdb.NewPostgresPool(ctx, platformdb.PostgresConfig{
		DataSourceName: dsn,
		MaxConns:       1,
	})
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	service := applicationsapp.Service{UOW: New(pgtenant.NewInstrumentedDBTX(pool))}
	detail, err := service.GetApplicationByOrgAndSlug(ctx, "built-in", "app-built-in")
	require.NoError(t, err)
	require.Equal(t, "01a0ea76-0c00-7000-8000-000000000002", detail.ID)
	require.Equal(t, "app-built-in", detail.Slug)
}
