package main

// GM 0-R3-T-L3: one-shot, idempotent, replayable backfill for organizations that
// predate the tenant identity/topology model. It writes the same 1:1 identity and
// primary isolation topology that CreateOrganization provisions, but never
// touches organizations themselves and never lives inside V1: V1 is the frozen
// structural baseline, this is an operator action.
//
// The scan only considers active organizations (deleted_at IS NULL) that are
// missing either guliangiam.tenant_identities or a non-decommissioned primary
// guliangiam.tenant_instances row. Writes are guarded by ON CONFLICT / NOT EXISTS
// so an interrupted or repeated run is safe; no checkpoint is needed because each
// run re-scans for what is still missing.

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
)

const (
	topologyBackfillZeroUUID   = "00000000-0000-0000-0000-000000000000"
	defaultTopologyBatch       = 200
	topologyBackfillRLSBypass  = `SELECT set_config('app.current_role', 'super_admin', true), set_config('app.current_organization_id', '', true)`
	topologyBackfillPrimarySQL = `
SELECT id::text, region_code, country_code
FROM guliangiam.deployment_regions
WHERE is_primary
ORDER BY region_code ASC, id ASC
LIMIT 1`

	topologyBackfillScanSQL = `
SELECT o.id::text, o.slug,
       (ti.organization_id IS NULL) AS missing_identity,
       (inst.id IS NULL) AS missing_topology
FROM guliangiam.organizations o
LEFT JOIN guliangiam.tenant_identities ti ON ti.organization_id = o.id
LEFT JOIN LATERAL (
    SELECT 1 AS id
    FROM guliangiam.tenant_instances candidate
    WHERE candidate.organization_id = o.id
      AND candidate.is_primary
      AND candidate.status <> 'decommissioned'
    LIMIT 1
) inst ON TRUE
WHERE o.deleted_at IS NULL
  AND (ti.organization_id IS NULL OR inst.id IS NULL)
  AND o.id > $1::uuid
ORDER BY o.id ASC
LIMIT $2`

	topologyBackfillAggregateSQL = `
SELECT
    count(*) AS affected_orgs,
    count(*) FILTER (WHERE ti.organization_id IS NULL) AS missing_identities,
    count(*) FILTER (WHERE inst.id IS NULL) AS missing_topologies
FROM guliangiam.organizations o
LEFT JOIN guliangiam.tenant_identities ti ON ti.organization_id = o.id
LEFT JOIN LATERAL (
    SELECT 1 AS id
    FROM guliangiam.tenant_instances candidate
    WHERE candidate.organization_id = o.id
      AND candidate.is_primary
      AND candidate.status <> 'decommissioned'
    LIMIT 1
) inst ON TRUE
WHERE o.deleted_at IS NULL
  AND (ti.organization_id IS NULL OR inst.id IS NULL)`

	topologyBackfillIdentitySQL = `
INSERT INTO guliangiam.tenant_identities (
    organization_id, status, isolation_tier, is_built_in, last_error,
    provisioned_at, decommissioned_at, created_at, updated_at, revision
) VALUES (
    $1::uuid, 'active', 'row', $2, NULL,
    now(), NULL, now(), now(), 1
)
ON CONFLICT (organization_id) DO NOTHING`

	topologyBackfillTopologySQL = `
INSERT INTO guliangiam.tenant_instances (
    id, organization_id, region_id, region_code, is_primary, status,
    placement_kind, storage_boundary, backup_boundary, data_residency_country,
    capacity_quota_units, provisioned_at, effective_from, created_at, updated_at, revision
)
SELECT $1::uuid, $2::uuid, $3::uuid, $4, true, 'active',
       'shared_cluster', 'shared', 'shared', $5,
       0, now(), now(), now(), now(), 1
WHERE NOT EXISTS (
    SELECT 1
    FROM guliangiam.tenant_instances existing
    WHERE existing.organization_id = $2::uuid
      AND existing.is_primary
      AND existing.status <> 'decommissioned'
)
ON CONFLICT (organization_id, region_id) DO UPDATE
SET is_primary = true,
    status = 'active',
    provisioned_at = now(),
    effective_from = now(),
    updated_at = now(),
    revision = guliangiam.tenant_instances.revision + 1`
)

type topologyBackfillOptions struct {
	BatchSize int
	DryRun    bool
	Verify    bool
}

type topologyBackfillStats struct {
	Scanned           int64
	AffectedOrgs      int64
	MissingIdentities int64
	MissingTopologies int64
	IdentityCreated   int64
	TopologyCreated   int64
	IdentitySkipped   int64
	TopologySkipped   int64
	Batches           int
	Elapsed           time.Duration
}

type topologyBackfillRow struct {
	id              string
	slug            string
	missingIdentity bool
	missingTopology bool
}

type topologyBackfillRegion struct {
	id         string
	regionCode string
	country    string
}

func runBackfillTenantTopology(argv []string) int {
	ctx := context.Background()

	fs := flag.NewFlagSet("migrate backfill-tenant-topology", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	batchSize := fs.Int("batch-size", defaultTopologyBatch, "organizations per scan/write batch")
	dryRun := fs.Bool("dry-run", false, "report missing identities/topologies without writing")
	verify := fs.Bool("verify", false, "fail if any active organization still lacks an identity or primary topology; never writes")
	if !parseFlags(fs, argv) {
		return 2
	}
	if *batchSize < 1 {
		fmt.Fprintln(os.Stderr, "-batch-size must be >= 1")
		return 2
	}
	if *dryRun && *verify {
		fmt.Fprintln(os.Stderr, "-dry-run and -verify are mutually exclusive")
		return 2
	}

	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	stats, code := backfillTenantTopologyWithDB(ctx, sqlDB, topologyBackfillOptions{
		BatchSize: *batchSize, DryRun: *dryRun, Verify: *verify,
	})
	printTopologyBackfillStats(stats, topologyBackfillOptions{
		BatchSize: *batchSize, DryRun: *dryRun, Verify: *verify,
	})
	return code
}

func backfillTenantTopologyWithDB(ctx context.Context, sqlDB *sql.DB, opts topologyBackfillOptions) (stats topologyBackfillStats, code int) {
	start := time.Now()
	defer func() { stats.Elapsed = time.Since(start) }()

	if opts.Verify || opts.DryRun {
		affected, missingIdentity, missingTopology, err := topologyBackfillAggregate(ctx, sqlDB)
		if err != nil {
			slog.Error("tenant topology backfill: aggregate scan failed", "err", err)
			return stats, 1
		}
		stats.AffectedOrgs = affected
		stats.MissingIdentities = missingIdentity
		stats.MissingTopologies = missingTopology
		stats.Scanned = affected
		if opts.Verify {
			if affected > 0 {
				slog.Error("tenant topology backfill: verify failed",
					"affected_orgs", affected,
					"missing_identities", missingIdentity,
					"missing_topologies", missingTopology)
				return stats, 1
			}
			slog.Info("tenant topology backfill: verify OK", "affected_orgs", 0)
			return stats, 0
		}
		slog.Info("tenant topology backfill: dry-run complete",
			"affected_orgs", affected,
			"would_create_identities", missingIdentity,
			"would_create_topologies", missingTopology,
			"batch_size", opts.BatchSize)
		return stats, 0
	}

	region, err := topologyBackfillPrimaryRegion(ctx, sqlDB)
	if err != nil {
		slog.Error("tenant topology backfill: read primary region failed", "err", err)
		return stats, 1
	}

	cursor := topologyBackfillZeroUUID
	for {
		batch, err := topologyBackfillBatch(ctx, sqlDB, cursor, opts.BatchSize, region)
		if err != nil {
			slog.Error("tenant topology backfill: batch failed", "batch", stats.Batches+1, "err", err)
			return stats, 1
		}
		if len(batch) == 0 {
			break
		}
		stats.Batches++
		stats.Scanned += int64(len(batch))
		var created int64
		for i := range batch {
			row := &batch[i]
			created += boolToInt64(row.identityCreated)
			created += boolToInt64(row.topologyCreated)
			stats.IdentityCreated += boolToInt64(row.identityCreated)
			stats.TopologyCreated += boolToInt64(row.topologyCreated)
			stats.IdentitySkipped += boolToInt64(row.missingIdentity && !row.identityCreated)
			stats.TopologySkipped += boolToInt64(row.missingTopology && !row.topologyCreated)
		}
		if created == 0 {
			// The scan predicate would return the same page again; stopping is
			// safer than looping forever when a row cannot be repaired.
			slog.Error("tenant topology backfill: batch made no progress; stopping",
				"batch", stats.Batches, "rows", len(batch))
			return stats, 1
		}
		cursor = batch[len(batch)-1].id
	}

	remaining, missingIdentity, missingTopology, err := topologyBackfillAggregate(ctx, sqlDB)
	if err != nil {
		slog.Error("tenant topology backfill: post-run scan failed", "err", err)
		return stats, 1
	}
	stats.AffectedOrgs = remaining
	stats.MissingIdentities = missingIdentity
	stats.MissingTopologies = missingTopology
	if remaining > 0 {
		slog.Error("tenant topology backfill: organizations still missing identity/topology",
			"affected_orgs", remaining,
			"missing_identities", missingIdentity,
			"missing_topologies", missingTopology)
		return stats, 1
	}
	slog.Info("tenant topology backfill: complete",
		"scanned_orgs", stats.Scanned,
		"identities_created", stats.IdentityCreated,
		"topologies_created", stats.TopologyCreated,
		"identities_skipped", stats.IdentitySkipped,
		"topologies_skipped", stats.TopologySkipped,
		"batches", stats.Batches, "region", region.regionCode)
	return stats, 0
}

type topologyBackfillBatchRow struct {
	topologyBackfillRow
	identityCreated bool
	topologyCreated bool
}

func topologyBackfillPrimaryRegion(ctx context.Context, sqlDB *sql.DB) (topologyBackfillRegion, error) {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return topologyBackfillRegion{}, fmt.Errorf("begin region read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, topologyBackfillRLSBypass); err != nil {
		return topologyBackfillRegion{}, fmt.Errorf("set rls context: %w", err)
	}
	var region topologyBackfillRegion
	err = tx.QueryRowContext(ctx, topologyBackfillPrimarySQL).Scan(&region.id, &region.regionCode, &region.country)
	if err != nil {
		if err == sql.ErrNoRows {
			return topologyBackfillRegion{}, fmt.Errorf("no primary deployment_region configured; cannot backfill topology")
		}
		return topologyBackfillRegion{}, fmt.Errorf("read primary region: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return topologyBackfillRegion{}, fmt.Errorf("commit region read: %w", err)
	}
	return region, nil
}

func topologyBackfillAggregate(ctx context.Context, sqlDB *sql.DB) (affected, missingIdentity, missingTopology int64, err error) {
	tx, err := sqlDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("begin aggregate read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, topologyBackfillRLSBypass); err != nil {
		return 0, 0, 0, fmt.Errorf("set rls context: %w", err)
	}
	if err := tx.QueryRowContext(ctx, topologyBackfillAggregateSQL).Scan(&affected, &missingIdentity, &missingTopology); err != nil {
		return 0, 0, 0, fmt.Errorf("read aggregate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, 0, fmt.Errorf("commit aggregate read: %w", err)
	}
	return affected, missingIdentity, missingTopology, nil
}

func topologyBackfillBatch(ctx context.Context, sqlDB *sql.DB, cursor string, batchSize int, region topologyBackfillRegion) ([]topologyBackfillBatchRow, error) {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin batch: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, topologyBackfillRLSBypass); err != nil {
		return nil, fmt.Errorf("set rls context: %w", err)
	}

	rows, err := tx.QueryContext(ctx, topologyBackfillScanSQL, cursor, batchSize)
	if err != nil {
		return nil, fmt.Errorf("scan organizations: %w", err)
	}
	var batch []topologyBackfillBatchRow
	for rows.Next() {
		var row topologyBackfillBatchRow
		if err := rows.Scan(&row.id, &row.slug, &row.missingIdentity, &row.missingTopology); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan organization: %w", err)
		}
		batch = append(batch, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate organizations: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close organizations: %w", err)
	}

	for i := range batch {
		row := &batch[i]
		if row.missingIdentity {
			result, err := tx.ExecContext(ctx, topologyBackfillIdentitySQL, row.id, row.slug == "built-in")
			if err != nil {
				return nil, fmt.Errorf("insert identity for organization %s: %w", row.id, err)
			}
			if n, err := result.RowsAffected(); err != nil {
				return nil, fmt.Errorf("read identity rows affected: %w", err)
			} else if n > 0 {
				row.identityCreated = true
			}
		}
		if row.missingTopology {
			result, err := tx.ExecContext(ctx, topologyBackfillTopologySQL,
				uuid.NewString(), row.id, region.id, region.regionCode, region.country)
			if err != nil {
				return nil, fmt.Errorf("insert primary topology for organization %s: %w", row.id, err)
			}
			if n, err := result.RowsAffected(); err != nil {
				return nil, fmt.Errorf("read topology rows affected: %w", err)
			} else if n > 0 {
				row.topologyCreated = true
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit batch: %w", err)
	}
	return batch, nil
}

func printTopologyBackfillStats(stats topologyBackfillStats, opts topologyBackfillOptions) {
	mode := "write"
	switch {
	case opts.DryRun:
		mode = "dry-run"
	case opts.Verify:
		mode = "verify"
	}
	fmt.Printf("tenant topology backfill (%s): scanned=%d affected=%d missing_identities=%d missing_topologies=%d identities_created=%d topologies_created=%d identities_skipped=%d topologies_skipped=%d batches=%d elapsed=%s\n",
		mode, stats.Scanned, stats.AffectedOrgs, stats.MissingIdentities, stats.MissingTopologies,
		stats.IdentityCreated, stats.TopologyCreated, stats.IdentitySkipped, stats.TopologySkipped,
		stats.Batches, stats.Elapsed.Round(time.Millisecond))
}

func boolToInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
