package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// MigrationKind separates structural DDL from data DML in one ordered model.
//
// The split is what keeps seeding out of the schema version chain (ADR-022 §6):
// a structure version and a seed version may carry the same number, they are
// gated by different switches (AUTO_MIGRATE / AUTO_SEED), and only structure
// versions participate in the "database is not newer than this image" check.
type MigrationKind string

const (
	// KindStructure is generated DDL: schemas, tables, indexes, constraints, RLS.
	KindStructure MigrationKind = "structure"
	// KindData is generated, idempotent seed DML.
	KindData MigrationKind = "data"
)

// Migration is one versioned, forward-only migration unit.
//
// Forward-only is deliberate: rollback is executed by the deploy pipeline from
// the generated `db/rollback/*.down.sql` artifacts, never by the running
// process. A process that can silently reverse a schema change is a production
// hazard, and the operator path (migration Job) already owns that step.
type Migration struct {
	Kind    MigrationKind
	Version int64
	Name    string
	SQL     string
}

// AppliedMigration is one ledger row read back from the database. It is
// exported so the operator CLI (`cmd/migrate status`) can report the structure
// version without duplicating the ledger schema in a second place.
type AppliedMigration struct {
	Kind        MigrationKind
	Version     int64
	Name        string
	Checksum    string
	AppliedAt   time.Time
	ExecutionMs int64
}

// ledgerDDL creates the migration ledger. It is the only object the application
// writes outside the declarative schema, and it is infrastructure owned by this
// package alone (the `guliangiam.schema_migrations` ledger); the declarative
// schema must never declare a table of the same name, which
// `scripts/check-migrations.sh` enforces.
const ledgerDDL = `
CREATE SCHEMA IF NOT EXISTS guliangiam;

CREATE TABLE IF NOT EXISTS guliangiam.schema_migrations (
    kind          TEXT        NOT NULL,
    version       BIGINT      NOT NULL,
    name          TEXT        NOT NULL,
    checksum      TEXT        NOT NULL,
    applied_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    execution_ms  BIGINT      NOT NULL,
    CONSTRAINT pk_schema_migrations PRIMARY KEY (kind, version)
);`

// Migrate brings the database to the version declared by the embedded
// migration set.
//
// Semantics (ADR-022 §3):
//
//   - every migration is applied in its own transaction together with its ledger
//     row, so a failure can never leave a half-applied version recorded;
//   - versions are applied in ascending order and applied versions are skipped,
//     so a restart, a second replica, or an operator replay is safe;
//   - an applied version whose content changed is refused outside development;
//     in development, its idempotent SQL is replayed and its checksum repaired;
//   - a ledger version that this image does not contain is refused: it means the
//     database is newer than the code (a rollback of the app without a rollback
//     of the schema), which must be resolved by the operator, not guessed here;
//   - structure and data are gated separately by AUTO_MIGRATE and AUTO_SEED, so
//     a production process can never seed or restructure by accident (DB-053).
//
// When AUTO_MIGRATE is off, the schema is only verified to exist so a
// misconfigured deployment fails with a clear message instead of a
// missing-relation error later.
func Migrate(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) error {
	ordered, err := validateMigrations(migrations)
	if err != nil {
		return err
	}

	autoMigrate := config.AutoMigrate()
	autoSeed := config.AutoSeed()
	allowChecksumDrift, err := checksumDriftAllowed()
	if err != nil {
		return err
	}

	if !autoMigrate {
		if err := verifySchemaPresent(ctx, pool); err != nil {
			return err
		}
	}
	if !autoMigrate && !autoSeed {
		return nil
	}

	// Optional pin: when SCHEMA_CONTENT_HASH is set, the embedded structure set
	// must match it. This catches an image built from a different revision than
	// the one the operator intends to apply.
	if want := config.SchemaContentHash(); want != "" {
		if got := StructureContentHash(ordered); !strings.EqualFold(got, want) {
			return apperrors.NewInvalidArgumentf(
				"schema content hash mismatch: image has %s but SCHEMA_CONTENT_HASH is %s", got, want)
		}
	}

	if _, err := pool.Exec(ctx, ledgerDDL); err != nil {
		return apperrors.WrapInternalf(err, "ensure migration ledger")
	}

	applied, err := readApplied(ctx, pool)
	if err != nil {
		return err
	}

	if allowChecksumDrift {
		slog.Warn("development checksum drift handling enabled; changed applied migrations will be replayed")
		ledger := make(map[string]AppliedMigration, len(applied))
		for _, a := range applied {
			ledger[fmt.Sprintf("%s/%d", a.Kind, a.Version)] = a
		}
		for _, m := range ordered {
			row, ok := ledger[fmt.Sprintf("%s/%d", m.Kind, m.Version)]
			if !ok {
				continue
			}
			if got, want := contentSHA256(m.SQL), row.Checksum; got != want {
				gateOpen := (m.Kind == KindStructure && autoMigrate) || (m.Kind == KindData && autoSeed)
				if !gateOpen {
					continue
				}
				if err := replayDriftedMigration(ctx, pool, m, got); err != nil {
					return err
				}
				slog.Info("changed migration replayed and checksum repaired",
					"kind", m.Kind, "version", m.Version, "name", m.Name)
			}
		}
		// Re-read the ledger after transactional replays before planning pending versions.
		applied, err = readApplied(ctx, pool)
		if err != nil {
			return err
		}
	}

	pending, err := planMigrations(ordered, applied, autoMigrate, autoSeed)
	if err != nil {
		return err
	}

	for _, m := range pending {
		if err := applyMigration(ctx, pool, m); err != nil {
			return err
		}
	}
	return nil
}

func checksumDriftAllowed() (bool, error) {
	if config.IsProduction() {
		if EnvTruthy("ALLOW_CHECKSUM_DRIFT") {
			return false, apperrors.NewInvalidArgument(
				"ALLOW_CHECKSUM_DRIFT is forbidden when APP_ENV=production")
		}
		return false, nil
	}
	if _, set := os.LookupEnv("ALLOW_CHECKSUM_DRIFT"); set {
		return EnvTruthy("ALLOW_CHECKSUM_DRIFT"), nil
	}
	return config.DriftImplicitlyAllowed(), nil
}

// StructureContentHash is the content pin of the structure chain: it changes
// whenever any structure migration changes, and never when seeds change.
func StructureContentHash(migrations []Migration) string {
	h := sha256.New()
	for _, m := range migrations {
		if m.Kind != KindStructure {
			continue
		}
		fmt.Fprintf(h, "%d:%s:%s\n", m.Version, m.Name, contentSHA256(m.SQL))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// validateMigrations rejects sets that cannot be applied deterministically.
// A non-deterministic order is how two replicas end up on different structures,
// so it is checked before anything touches the database.
func validateMigrations(migrations []Migration) ([]Migration, error) {
	if len(migrations) == 0 {
		return nil, apperrors.NewNotFound("migration set is empty")
	}

	seen := make(map[string]struct{}, len(migrations))
	ordered := make([]Migration, len(migrations))
	copy(ordered, migrations)

	for _, m := range ordered {
		if m.SQL == "" {
			return nil, apperrors.NewInvalidArgumentf("migration %s V%d has empty SQL", m.Kind, m.Version)
		}
		if m.Version <= 0 {
			return nil, apperrors.NewInvalidArgumentf("migration %s %q has non-positive version", m.Kind, m.Name)
		}
		if strings.TrimSpace(m.Name) == "" {
			return nil, apperrors.NewInvalidArgumentf("migration %s V%d has empty name", m.Kind, m.Version)
		}
		switch m.Kind {
		case KindStructure, KindData:
		default:
			return nil, apperrors.NewInvalidArgumentf("migration has unknown kind %q", m.Kind)
		}

		key := fmt.Sprintf("%s/%d", m.Kind, m.Version)
		if _, dup := seen[key]; dup {
			return nil, apperrors.NewInvalidArgumentf("duplicate migration %s V%d", m.Kind, m.Version)
		}
		seen[key] = struct{}{}
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Kind != ordered[j].Kind {
			return ordered[i].Kind == KindStructure
		}
		return ordered[i].Version < ordered[j].Version
	})
	return ordered, nil
}

// planMigrations computes what still has to run and refuses the two drift
// conditions that must never be resolved automatically.
func planMigrations(
	migrations []Migration, applied []AppliedMigration, autoMigrate, autoSeed bool,
) ([]Migration, error) {
	recorded := make(map[string]AppliedMigration, len(applied))
	for _, a := range applied {
		recorded[fmt.Sprintf("%s/%d", a.Kind, a.Version)] = a
	}

	present := make(map[string]struct{}, len(migrations))
	pending := make([]Migration, 0, len(migrations))

	for _, m := range migrations {
		key := fmt.Sprintf("%s/%d", m.Kind, m.Version)
		present[key] = struct{}{}

		row, ok := recorded[key]
		if !ok {
			// A closed gate means "this process must not apply it": structure is
			// gated by AUTO_MIGRATE, data by AUTO_SEED.
			if (m.Kind == KindStructure && autoMigrate) || (m.Kind == KindData && autoSeed) {
				pending = append(pending, m)
			}
			continue
		}

		if got, want := contentSHA256(m.SQL), row.Checksum; got != want {
			return nil, apperrors.NewInvalidArgumentf(
				"%s migration V%d (%s) was already applied with checksum %s but the image carries %s; "+
					"an applied migration must never be edited — add a new version instead",
				m.Kind, m.Version, m.Name, want, got)
		}
	}

	// Any ledger row absent from this image is a version mismatch. Silently
	// skipping it would let an older image start against a newer schema.
	for _, a := range applied {
		key := fmt.Sprintf("%s/%d", a.Kind, a.Version)
		if _, ok := present[key]; ok {
			continue
		}
		if a.Kind == KindStructure && a.Version == 1 {
			return nil, apperrors.NewInvalidArgumentf(
				"database ledger missing structure V1 — the schema_migrations table may be corrupted; " +
					"re-run the full initial DDL before starting")
		}
		return nil, apperrors.NewInvalidArgumentf(
			"database contains %s migration V%d (%s) not present in this image; "+
				"this image does not contain that version, so resolve the version mismatch before starting",
			a.Kind, a.Version, a.Name)
	}

	return pending, nil
}

func replayDriftedMigration(ctx context.Context, pool *pgxpool.Pool, m Migration, checksum string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return apperrors.WrapInternalf(err, "begin drifted %s migration V%d replay", m.Kind, m.Version)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if err := bypassRLSForDataMigration(ctx, tx, m); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return apperrors.WrapInternalf(err, "replay drifted %s migration V%d (%s)", m.Kind, m.Version, m.Name)
	}
	tag, err := tx.Exec(ctx,
		`UPDATE guliangiam.schema_migrations SET checksum = $1
		  WHERE kind = $2 AND version = $3`,
		checksum, string(m.Kind), m.Version,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "repair checksum for %s migration V%d", m.Kind, m.Version)
	}
	if tag.RowsAffected() != 1 {
		return apperrors.NewInvalidArgumentf(
			"cannot repair %s migration V%d checksum: expected one ledger row, updated %d",
			m.Kind, m.Version, tag.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return apperrors.WrapInternalf(err, "commit drifted %s migration V%d replay", m.Kind, m.Version)
	}
	committed = true
	return nil
}

// applyMigration runs one migration and records it atomically: the ledger row
// commits in the same transaction as the migration itself, so the ledger can
// never claim a version that was not applied.
func applyMigration(ctx context.Context, pool *pgxpool.Pool, m Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return apperrors.WrapInternalf(err, "begin %s migration V%d", m.Kind, m.Version)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// RLS bypass for data migrations — seed INSERTs happen before any app session,
	// so RLS policies that gate on app.current_organization_id would evaluate
	// to fail-closed. SELECT set_config is used (not SET LOCAL) because the
	// custom namespace is app.*, not a regular GUC that SET LOCAL accepts.
	if err := bypassRLSForDataMigration(ctx, tx, m); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return apperrors.WrapInternalf(err, "execute %s migration V%d (%s)", m.Kind, m.Version, m.Name)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO guliangiam.schema_migrations (kind, version, name, checksum, execution_ms)
		 VALUES ($1, $2, $3, $4, 0)
		 ON CONFLICT (kind, version) DO NOTHING`,
		string(m.Kind), m.Version, m.Name, contentSHA256(m.SQL),
	); err != nil {
		return apperrors.WrapInternalf(err, "record %s migration V%d in ledger", m.Kind, m.Version)
	}

	if err := tx.Commit(ctx); err != nil {
		return apperrors.WrapInternalf(err, "commit %s migration V%d", m.Kind, m.Version)
	}
	committed = true

	return nil
}

func bypassRLSForDataMigration(ctx context.Context, tx pgx.Tx, m Migration) error {
	if m.Kind != KindData {
		return nil
	}
	if _, err := tx.Exec(ctx, `
			SELECT set_config('app.current_organization_id', '', true);
			SELECT set_config('app.current_role', 'super_admin', true);
		`); err != nil {
		return apperrors.WrapInternalf(err, "rls bypass for %s migration V%d", m.Kind, m.Version)
	}
	return nil
}

// readApplied loads the ledger. A missing ledger table means nothing was ever
// applied, which is not an error.
func readApplied(ctx context.Context, pool *pgxpool.Pool) ([]AppliedMigration, error) {
	rows, err := pool.Query(ctx,
		`SELECT kind, version, name, checksum, applied_at, execution_ms
		   FROM guliangiam.schema_migrations ORDER BY kind, version`)
	if err != nil {
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() == "42P01" {
			return nil, nil
		}
		return nil, apperrors.WrapInternalf(err, "read migration ledger")
	}
	defer rows.Close()

	var applied []AppliedMigration
	for rows.Next() {
		var a AppliedMigration
		var kind string
		if err := rows.Scan(&kind, &a.Version, &a.Name, &a.Checksum, &a.AppliedAt, &a.ExecutionMs); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan migration ledger")
		}
		a.Kind = MigrationKind(kind)
		applied = append(applied, a)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "iterate migration ledger")
	}
	return applied, nil
}

// Applied reports the versions recorded in the migration ledger, ordered by
// kind then version. A missing ledger table reads as "nothing applied" rather
// than an error, so the caller can report an uninitialised database.
func Applied(ctx context.Context, pool *pgxpool.Pool) ([]AppliedMigration, error) {
	return readApplied(ctx, pool)
}

// verifySchemaPresent fails fast when the schema was never applied, so a
// misconfigured deployment reports a clear error instead of failing later on a
// missing relation.
func verifySchemaPresent(ctx context.Context, pool *pgxpool.Pool) error {
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'guliangiam'`,
	).Scan(&count); err != nil {
		return apperrors.WrapInternalf(err, "schema verification failed")
	}
	if count == 0 {
		return apperrors.NewNotFound(
			"schema is empty and AUTO_MIGRATE is not enabled; run `cmd/migrate up` to apply embedded V1+ migrations from internal/bootstrap/schema_sql/")
	}
	return nil
}

// EnvTruthy reports whether the named environment variable is explicitly set to
// a truthy value. Only "true"/"1"/"yes"/"on" (case-insensitive) enable a
// feature; unset and every other value are off.
func EnvTruthy(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func contentSHA256(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
