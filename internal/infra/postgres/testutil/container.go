//go:build integration

package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer wraps a PostgreSQL database for integration tests. It is
// either a freshly started testcontainers instance or an externally supplied
// database (see SetupPostgres).
type PostgresContainer struct {
	Container testcontainers.Container // nil when an external DSN was used
	Dsn       string                   // connection string for pgxpool
	external  bool                     // true when the caller owns the database
}

// schemaPaths resolves the DDL and seed files. Structure artifact lives in
// internal/bootstrap/schema_sql/ and seed in internal/bootstrap/seed_sql/ (they
// are separate version chains, ADR-022 §6). Embedded via go:embed in migrate.go,
// so tests don't depend on a flat directory layout anymore.
func schemaPaths(t *testing.T) (string, string) {
	t.Helper()

	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	schemaPath := filepath.Join(projectRoot, "db", "postgresql", "V1__guliangiam_schema_init.sql")
	seedPath := filepath.Join(projectRoot, "db", "seeds", "V1__guliangiam_seed_init.sql")

	if _, err := os.Stat(schemaPath); err != nil {
		t.Fatalf("schema file not found at %s: %v", schemaPath, err)
	}
	return schemaPath, seedPath
}

// Fixed ids the repository integration cases reference. They are exported so
// the fixtures below and the tests cannot drift apart.
const (
	TestOrganizationID = "01900000-0000-7000-8000-000000000200"
	TestUserID         = "01900000-0000-7000-8000-000000000100"
	TestApplicationID  = "01900000-0000-7000-8000-000000000300"
)

// SetupPostgres returns a database for integration tests.
//
// When POSTGRES_TEST_DSN is set the test runs against that database (the schema
// is replayed into it) and nothing is torn down; otherwise a throwaway
// testcontainers PostgreSQL instance is started. The env var path is what lets
// the suite run in environments without a Docker daemon.
func SetupPostgres(ctx context.Context, t *testing.T) *PostgresContainer {
	t.Helper()

	schemaPath, seedPath := schemaPaths(t)

	if dsn := config.PostgresTestDSN(); dsn != "" {
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatalf("failed to open external db: %v", err)
		}
		defer db.Close()

		requireRLSBypass(ctx, t, db)

		if err := executeSQLFile(db, schemaPath); err != nil {
			t.Fatalf("failed to execute V1 schema: %v", err)
		}
		if err := executeSQLFile(db, seedPath); err != nil {
			t.Logf("warning: failed to execute V2 seed (non-fatal): %v", err)
		}

		t.Logf("integration tests running against POSTGRES_TEST_DSN")
		return &PostgresContainer{Dsn: dsn, external: true}
	}

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("guliangiam"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := executeSQLFile(db, schemaPath); err != nil {
		t.Fatalf("failed to execute V1 schema: %v", err)
	}

	if err := executeSQLFile(db, seedPath); err != nil {
		t.Logf("warning: failed to execute V2 seed (non-fatal): %v", err)
	}

	return &PostgresContainer{
		Container: pgContainer,
		Dsn:       dsn,
	}
}

// ConnectPool creates a pgxpool.Pool connected to the test database.
func (pc *PostgresContainer) ConnectPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()

	config, err := pgxpool.ParseConfig(pc.Dsn)
	if err != nil {
		t.Fatalf("failed to parse pool config: %v", err)
	}
	config.MinConns = 2
	config.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping: %v", err)
	}

	return pool
}

// Cleanup terminates the container. An externally supplied database is left
// alone: the caller owns it.
func (pc *PostgresContainer) Cleanup(ctx context.Context) {
	if pc.external {
		return
	}
	if pc.Container != nil {
		pc.Container.Terminate(ctx) //nolint:errcheck
	}
}

// TruncateAll truncates all tables in the guliangiam schema for clean test isolation.
func TruncateAll(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `DO $$
		DECLARE tbl RECORD;
		BEGIN
			FOR tbl IN
				SELECT tablename FROM pg_tables WHERE schemaname = 'guliangiam'
			LOOP
				EXECUTE format('TRUNCATE TABLE guliangiam.%I CASCADE', tbl.tablename);
			END LOOP;
		END; $$`)
	return err
}

// TruncateAllAndSeedOrganization truncates every table and then re-inserts the
// organization that the repository integration cases reference by fixed id.
// Without it, TRUNCATE CASCADE removes the seeded organization and every child
// insert fails with 23503 (fk_*_organization_id_organizations).
func TruncateAllAndSeedOrganization(ctx context.Context, pool *pgxpool.Pool) error {
	if err := TruncateAll(ctx, pool); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, `INSERT INTO guliangiam.organizations (id, slug, display_name)
		VALUES ($1::uuid, 'test-org', 'Test Org') ON CONFLICT (id) DO NOTHING`, TestOrganizationID)
	return err
}

// TruncateAllAndSeedSessionPrerequisites additionally seeds the user and
// application rows that the sessions and tokens cases reference by fixed id
// (sessions.user_id / sessions.application_id are NOT NULL foreign keys).
func TruncateAllAndSeedSessionPrerequisites(ctx context.Context, pool *pgxpool.Pool) error {
	if err := TruncateAllAndSeedOrganization(ctx, pool); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `INSERT INTO guliangiam.users (id, organization_id, username)
		VALUES ($1::uuid, $2::uuid, 'testuser') ON CONFLICT (id) DO NOTHING`, TestUserID, TestOrganizationID); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, `INSERT INTO guliangiam.applications (id, organization_id, slug, display_name)
		VALUES ($1::uuid, $2::uuid, 'test-app', 'Test App') ON CONFLICT (id) DO NOTHING`, TestApplicationID, TestOrganizationID)
	return err
}

// requireRLSBypass fails fast when the external database is reached with a role
// that cannot bypass row-level security. The guliangiam tables use
// FORCE ROW LEVEL SECURITY and the tests talk to a raw pgxpool (no
// platformdb tenant-scope PrepareConn), so a plain non-superuser role makes
// every insert fail with SQLSTATE 42501 before any assertion runs.
func requireRLSBypass(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	var isSuper, bypassRLS bool
	if err := db.QueryRowContext(ctx,
		`SELECT rolsuper, rolbypassrls FROM pg_catalog.pg_roles WHERE rolname = current_user`).Scan(&isSuper, &bypassRLS); err != nil {
		t.Fatalf("failed to inspect POSTGRES_TEST_DSN role privileges: %v", err)
	}
	if !isSuper && !bypassRLS {
		t.Fatalf("POSTGRES_TEST_DSN role must be superuser or have BYPASSRLS: " +
			"the repository integration tests use a raw pgxpool without tenant GUCs, " +
			"and guliangiam tables enforce RLS (SQLSTATE 42501 otherwise)")
	}
}

func executeSQLFile(db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file %s: %w", path, err)
	}

	if len(data) == 0 {
		return nil
	}

	_, err = db.Exec(string(data))
	if err != nil {
		return fmt.Errorf("execute SQL from %s: %w", path, err)
	}

	return nil
}
