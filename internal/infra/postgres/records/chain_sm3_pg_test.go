// Real-PostgreSQL evidence for the SM3 audit hash chain (GM AC-CRYPTO-003).
//
// Skipped unless GIAM_TEST_PG_DSN points at an already-migrated disposable
// database (V1→V10). Run it with:
//
//	GIAM_TEST_PG_DSN='postgres://.../guliangiam_sm3_verify?sslmode=disable' \
//	  GOCACHE=/tmp/giam-r2-cache go test ./internal/infra/postgres/records \
//	  -run TestAuditChainSM3Postgres -count=1 -v
//
// The DSN must belong to a role that bypasses the FORCE RLS policies (migration
// role, table owner, or superuser): the fixture and the verifier read/write the
// chain without a tenant GUC. The test creates only its own organizations and
// chains, and deletes exactly those rows afterwards.
//
// The fixture writes through the production repository (and therefore through
// AuditChainWriter.Compute), not through hand-built INSERTs, so the persisted
// prev_hash/entry_hash/chain_algo values are the ones the runtime writes. The
// verifier loads pages through ListChainRecordsBatch and recomputes entry_hash
// with VerifyChainRecordBatch — the same path the audit-chain worker uses.
package records

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

const (
	sm3PGChainPageSize  = 2
	sm3PGFixtureEntries = 4
)

type chainVerification struct {
	Verified int
	BreakSeq int64
	Err      error
}

func TestAuditChainSM3Postgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL SM3 audit-chain test")
	}
	// A new chain inherits its first algorithm from the deployment profile. The
	// fixture relies on the production selector: gm must produce sm3.
	t.Setenv("CRYPTO_PROFILE", "gm")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	var orgIDs []string
	t.Cleanup(func() {
		defer pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		for _, orgID := range orgIDs {
			// records.organization_id cascades, but deleting explicitly keeps the
			// cleanup idempotent even if the schema's FK changes.
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.records WHERE organization_id = $1::uuid`, orgID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, orgID)
		}
	})
	require.NoError(t, pool.Ping(ctx))

	repo := NewRepository(pool)
	writer := audit.NewAuditChainWriter(pool, DefaultChainSQL())

	orgID := newSM3PGOrg(t, ctx, pool, "sm3-verify")
	orgIDs = append(orgIDs, orgID)
	recordIDs := createSM3PGChain(t, ctx, repo, orgID, sm3PGFixtureEntries)

	t.Run("write and verify the full sm3 chain", func(t *testing.T) {
		rows, minSeq, maxSeq, sm3Rows := persistedChainStats(t, ctx, pool, orgID)
		require.Equal(t, int64(sm3PGFixtureEntries), rows)
		require.Equal(t, int64(1), minSeq)
		require.Equal(t, int64(sm3PGFixtureEntries), maxSeq)
		require.Equal(t, rows, sm3Rows, "every fixture row must persist chain_algo='sm3'")

		// The gm profile selects SM3 for a new chain, and the dispatch really
		// computes a different digest — not the sha256 baseline under a new label.
		require.Equal(t, platformcrypto.ChainAlgoSM3, platformcrypto.DefaultChainAlgorithm(),
			"CRYPTO_PROFILE=gm must select sm3 for the first chain entry")
		sm3Digest, err := platformcrypto.HashByChainAlgorithm(platformcrypto.ChainAlgoSM3, []byte("gm-profile-dispatch"))
		require.NoError(t, err)
		sha256Digest, err := platformcrypto.HashByChainAlgorithm(platformcrypto.ChainAlgoSHA256, []byte("gm-profile-dispatch"))
		require.NoError(t, err)
		require.NotEqual(t, sm3Digest, sha256Digest, "sm3 must not alias the sha256 digest")

		linked, linkErr := writer.VerifyChain(ctx, orgID)
		require.NoError(t, linkErr)
		require.Equal(t, sm3PGFixtureEntries, linked)

		got := verifySM3PGChain(t, ctx, writer, orgID, sm3PGChainPageSize)
		require.NoError(t, got.Err)
		require.Equal(t, sm3PGFixtureEntries, got.Verified)
		require.Zero(t, got.BreakSeq)
		t.Logf("chain_id=%s rows=%d chain_algo=sm3 verified=%d (page_size=%d)",
			orgID, rows, got.Verified, sm3PGChainPageSize)
	})

	t.Run("single payload tamper is rejected and located by record id", func(t *testing.T) {
		tamperedSeq := int64(2)
		tamperedID := recordIDs[tamperedSeq-1]
		_, err := pool.Exec(ctx, `UPDATE guliangiam.records SET action = 'audit.tampered' WHERE id = $1::uuid`, tamperedID)
		require.NoError(t, err)

		got := verifySM3PGChain(t, ctx, writer, orgID, sm3PGChainPageSize)
		require.Error(t, got.Err)
		require.Equal(t, 1, got.Verified, "entries before the break must stay verified")
		require.Equal(t, tamperedSeq, got.BreakSeq)
		require.Equal(t, tamperedID, recordIDAtSeq(t, ctx, pool, orgID, got.BreakSeq),
			"the breakpoint must resolve to the tampered record id")
		require.Contains(t, got.Err.Error(), "entry_hash mismatch with sm3")
		require.Contains(t, got.Err.Error(), "record tampered")
		t.Logf("payload tamper: breakpoint_seq=%d record_id=%s verified_before=%d err=%v",
			got.BreakSeq, tamperedID, got.Verified, got.Err)

		_, err = pool.Exec(ctx, `UPDATE guliangiam.records SET action = $1 WHERE id = $2::uuid`,
			sm3PGAction(tamperedSeq), tamperedID)
		require.NoError(t, err)
		restored := verifySM3PGChain(t, ctx, writer, orgID, sm3PGChainPageSize)
		require.NoError(t, restored.Err)
		require.Equal(t, sm3PGFixtureEntries, restored.Verified)
	})

	t.Run("single entry_hash tamper is rejected and located by record id", func(t *testing.T) {
		tamperedSeq := int64(3)
		tamperedID := recordIDs[tamperedSeq-1]
		var originalHash []byte
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT entry_hash FROM guliangiam.records WHERE id = $1::uuid`, tamperedID).Scan(&originalHash))
		require.Len(t, originalHash, 32)

		_, err := pool.Exec(ctx,
			`UPDATE guliangiam.records SET entry_hash = decode(repeat('ab', 32), 'hex') WHERE id = $1::uuid`,
			tamperedID)
		require.NoError(t, err)

		got := verifySM3PGChain(t, ctx, writer, orgID, sm3PGChainPageSize)
		require.Error(t, got.Err)
		require.Equal(t, 2, got.Verified)
		require.Equal(t, tamperedSeq, got.BreakSeq)
		require.Equal(t, tamperedID, recordIDAtSeq(t, ctx, pool, orgID, got.BreakSeq),
			"the breakpoint must resolve to the tampered record id")
		require.Contains(t, got.Err.Error(), "entry_hash mismatch with sm3")
		t.Logf("hash tamper: breakpoint_seq=%d record_id=%s verified_before=%d err=%v",
			got.BreakSeq, tamperedID, got.Verified, got.Err)

		_, err = pool.Exec(ctx, `UPDATE guliangiam.records SET entry_hash = $1 WHERE id = $2::uuid`,
			originalHash, tamperedID)
		require.NoError(t, err)
		restored := verifySM3PGChain(t, ctx, writer, orgID, sm3PGChainPageSize)
		require.NoError(t, restored.Err)
		require.Equal(t, sm3PGFixtureEntries, restored.Verified)
	})

	t.Run("mixed algorithm chain is rejected at the switch seq", func(t *testing.T) {
		mixedOrgID := newSM3PGOrg(t, ctx, pool, "sm3-mixed")
		orgIDs = append(orgIDs, mixedOrgID)
		mixedIDs := createSM3PGChain(t, ctx, repo, mixedOrgID, 3)

		switchSeq := int64(2)
		_, err := pool.Exec(ctx,
			`UPDATE guliangiam.records SET chain_algo = 'sha256' WHERE id = $1::uuid`,
			mixedIDs[switchSeq-1])
		require.NoError(t, err, "ck_records_chain_algo admits sha256, so a mixed chain is persistable")

		got := verifySM3PGChain(t, ctx, writer, mixedOrgID, sm3PGChainPageSize)
		require.Error(t, got.Err)
		require.Equal(t, 1, got.Verified)
		require.Equal(t, switchSeq, got.BreakSeq)
		require.Equal(t, mixedIDs[switchSeq-1], recordIDAtSeq(t, ctx, pool, mixedOrgID, got.BreakSeq))
		require.Contains(t, got.Err.Error(), "chain_algo changed from sm3 to sha256")
		require.Contains(t, got.Err.Error(), "mixing algorithms in one chain is forbidden")
		t.Logf("mixed chain: breakpoint_seq=%d record_id=%s verified_before=%d err=%v",
			got.BreakSeq, mixedIDs[switchSeq-1], got.Verified, got.Err)

		_, err = pool.Exec(ctx,
			`UPDATE guliangiam.records SET chain_algo = 'sm3' WHERE id = $1::uuid`,
			mixedIDs[switchSeq-1])
		require.NoError(t, err)
		restored := verifySM3PGChain(t, ctx, writer, mixedOrgID, sm3PGChainPageSize)
		require.NoError(t, restored.Err)
		require.Equal(t, 3, restored.Verified)
	})

	t.Run("unknown algorithm is rejected without a sha256 fallback", func(t *testing.T) {
		// The persisted closed set is enforced by the schema, not just by code.
		_, err := pool.Exec(ctx,
			`UPDATE guliangiam.records SET chain_algo = 'gost' WHERE id = $1::uuid`, recordIDs[0])
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "23514", pgErr.Code)
		require.Equal(t, "ck_records_chain_algo", pgErr.ConstraintName)
		t.Logf("persisting chain_algo='gost' rejected: sqlstate=%s constraint=%s", pgErr.Code, pgErr.ConstraintName)

		normalized, err := platformcrypto.NormalizeChainAlgorithm("gost")
		require.Error(t, err)
		require.Empty(t, normalized, "an unknown algorithm must not normalize to a default")
		sum, err := platformcrypto.HashByChainAlgorithm("gost", []byte("chain payload"))
		require.Error(t, err)
		require.Nil(t, sum, "an unknown algorithm must not fall back to sha256")

		// Recompute path: a row carrying gost is rejected, not hashed as sha256.
		records, err := writer.ListChainRecordsBatch(ctx, orgID, 0, sm3PGFixtureEntries)
		require.NoError(t, err)
		require.Len(t, records, sm3PGFixtureEntries)
		records[0].ChainAlgo = "gost"
		verified, err := audit.VerifyChainRecords(orgID, records)
		require.Error(t, err)
		require.Zero(t, verified)
		require.Contains(t, err.Error(), `unsupported audit chain algorithm "gost"`)
		t.Logf("gost recompute rejected: verified=%d err=%v", verified, err)

		// The failed UPDATE rolled back on its own; the real chain is intact.
		got := verifySM3PGChain(t, ctx, writer, orgID, sm3PGChainPageSize)
		require.NoError(t, got.Err)
		require.Equal(t, sm3PGFixtureEntries, got.Verified)
	})
}

func newSM3PGOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string) string {
	t.Helper()
	orgID := uuid.NewString()
	slug := label + "-" + strings.ReplaceAll(orgID, "-", "")
	_, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		orgID, slug, "sm3 audit-chain integration")
	require.NoError(t, err)
	return orgID
}

// createSM3PGChain writes entries through the production repository. The first
// record is hashed with CRYPTO_PROFILE=gm (sm3); every later record inherits the
// persisted tail algorithm, matching the runtime contract.
func createSM3PGChain(t *testing.T, ctx context.Context, repo Repository, orgID string, entries int) []string {
	t.Helper()
	base := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	ids := make([]string, 0, entries)
	for seq := 1; seq <= entries; seq++ {
		id := uuid.NewString()
		record := &domain.Record{
			ID:             id,
			OrganizationID: orgID,
			Action:         sm3PGAction(int64(seq)),
			ResourceType:   "audit",
			ResourceID:     fmt.Sprintf("sm3-resource-%d", seq),
			UserAgent:      "sm3-pg-integration",
			Surface:        domain.SurfaceAuthentication,
			// String values only: jsonb round-trips them verbatim, so the
			// canonical metadata payload cannot drift on number formatting.
			Metadata:  map[string]any{"run": "sm3-pg-integration", "seq": strconv.Itoa(seq)},
			CreatedAt: base.Add(time.Duration(seq) * time.Minute),
		}
		require.NoError(t, repo.Create(ctx, record))
		ids = append(ids, id)
	}
	return ids
}

func sm3PGAction(seq int64) string {
	return fmt.Sprintf("sm3.action.%d", seq)
}

func persistedChainStats(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chainID string) (rows, minSeq, maxSeq, sm3Rows int64) {
	t.Helper()
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(min(chain_seq), 0), COALESCE(max(chain_seq), 0),
		       count(*) FILTER (WHERE chain_algo = 'sm3')
		FROM guliangiam.records
		WHERE chain_id = $1`, chainID).Scan(&rows, &minSeq, &maxSeq, &sm3Rows))
	return rows, minSeq, maxSeq, sm3Rows
}

func recordIDAtSeq(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chainID string, seq int64) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM guliangiam.records WHERE chain_id = $1 AND chain_seq = $2`, chainID, seq).Scan(&id))
	require.NotEmpty(t, id)
	return id
}

// verifySM3PGChain streams the chain in bounded pages and runs the same pure
// verifier as the audit-chain worker and cmd/audit-verify. Verified counts only
// entries before the first failure; BreakSeq is the failing record's chain_seq.
func verifySM3PGChain(t *testing.T, ctx context.Context, writer *audit.AuditChainWriter, chainID string, pageSize int) chainVerification {
	t.Helper()
	var out chainVerification
	state := audit.ChainVerifyState{}
	afterSeq := int64(0)
	for {
		batch, err := writer.ListChainRecordsBatch(ctx, chainID, afterSeq, pageSize)
		if err != nil {
			out.Err = err
			return out
		}
		count, next, verifyErr := audit.VerifyChainRecordBatch(chainID, batch, state)
		out.Verified += count
		if verifyErr != nil {
			out.Err = verifyErr
			if count < len(batch) {
				out.BreakSeq = batch[count].ChainSeq
			}
			return out
		}
		state = next
		if len(batch) == 0 || len(batch) < pageSize {
			return out
		}
		afterSeq = batch[len(batch)-1].ChainSeq
	}
}
