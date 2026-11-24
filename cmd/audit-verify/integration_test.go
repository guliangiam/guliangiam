// Integration test for cmd/audit-verify against a real PostgreSQL audit chain.
//
// Skipped unless AUDIT_VERIFY_TEST_DSN points at an already-migrated disposable
// database (V1→V9). Run it with:
//
//	AUDIT_VERIFY_TEST_DSN=postgres://.../guliangiam_audit_crypto003_20261103 \
//	  go test ./cmd/audit-verify/... -run TestVerifyChainIntegration -count=1
//
// The DSN must be a role that can bypass the FORCE RLS policies (migration role,
// table owner, or superuser), because the fixture and the operator command both
// read/write across chain rows without a tenant GUC. The test creates one
// organization and two isolated chains (SM3 and SHA-256), tampers with one row at
// a time, restores each, and deletes only what it created.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

const insertAuditRecordSQL = `
INSERT INTO guliangiam.records
	(id, organization_id, action, resource_type, resource_id, user_agent, surface, metadata,
	 created_at, chain_id, chain_seq, prev_hash, entry_hash, chain_status, chain_algo)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'chained', $14)`

func TestVerifyChainIntegration(t *testing.T) {
	dsn := os.Getenv("AUDIT_VERIFY_TEST_DSN")
	if dsn == "" {
		t.Skip("AUDIT_VERIFY_TEST_DSN is not set; skipping PostgreSQL audit-chain integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var orgID string
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		defer pool.Close()
		if orgID == "" {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.records WHERE organization_id = $1::uuid`, orgID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, orgID)
	})
	require.NoError(t, pool.Ping(ctx))

	orgID = uuid.NewString()
	sm3ChainID := orgID
	sha256ChainID := orgID + "-sha256"

	_, err = pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		orgID, "audit-crypto003-"+strings.ReplaceAll(orgID, "-", ""), "audit-verify integration",
	)
	require.NoError(t, err)

	insertChain(t, pool, orgID, sm3ChainID, platformcrypto.ChainAlgoSM3, 3)
	insertChain(t, pool, orgID, sha256ChainID, platformcrypto.ChainAlgoSHA256, 2)

	// A: the intact SM3 chain verifies through the same seam the command uses.
	verified, err := verifyChain(ctx, dsn, sm3ChainID)
	require.NoError(t, err)
	require.Equal(t, 3, verified)

	// B: tamper only the second record. VerifyChainRecords returns the number of
	// entries verified BEFORE the break, so a seq-2 break yields 1 (the command
	// prints "INVALID after 1 verified entries"). It names the position "seq 2"
	// ("chain <id> seq 2: entry_hash mismatch with sm3 …"); the batch may not
	// change platform wording, so the assertions pin that actual text.
	tamperedSeq := 2
	_, err = pool.Exec(ctx,
		`UPDATE guliangiam.records SET action = 'audit.tampered' WHERE chain_id = $1 AND chain_seq = $2`, sm3ChainID, tamperedSeq)
	require.NoError(t, err)

	verified, err = verifyChain(ctx, dsn, sm3ChainID)
	require.Error(t, err)
	require.Equal(t, 1, verified)
	require.Contains(t, err.Error(), "seq 2:")
	require.Contains(t, err.Error(), "entry_hash mismatch")
	require.Contains(t, err.Error(), platformcrypto.ChainAlgoSM3)
	var invalid *chainInvalidError
	require.ErrorAs(t, err, &invalid, "a tampered chain must map to the exit-1 error type")
	require.Equal(t, 1, invalid.verified)

	// Restoring the original action closes the chain again.
	restoreAction(t, pool, sm3ChainID, tamperedSeq)
	verified, err = verifyChain(ctx, dsn, sm3ChainID)
	require.NoError(t, err)
	require.Equal(t, 3, verified)

	// B2: a seq-3 break returns 2 verified entries (the "INVALID after 2" form),
	// pinning the count semantics on the last entry.
	tamperedSeq = 3
	_, err = pool.Exec(ctx,
		`UPDATE guliangiam.records SET action = 'audit.tampered' WHERE chain_id = $1 AND chain_seq = $2`, sm3ChainID, tamperedSeq)
	require.NoError(t, err)
	verified, err = verifyChain(ctx, dsn, sm3ChainID)
	require.Error(t, err)
	require.Equal(t, 2, verified)
	require.Contains(t, err.Error(), "seq 3:")
	require.Contains(t, err.Error(), platformcrypto.ChainAlgoSM3)
	restoreAction(t, pool, sm3ChainID, tamperedSeq)
	verified, err = verifyChain(ctx, dsn, sm3ChainID)
	require.NoError(t, err)
	require.Equal(t, 3, verified)

	// C: a SHA-256 chain in the same table is a control group for algorithm dispatch.
	verified, err = verifyChain(ctx, dsn, sha256ChainID)
	require.NoError(t, err)
	require.Equal(t, 2, verified)
}

func actionFor(algo string, seq int) string { return fmt.Sprintf("%s.action.%d", algo, seq) }

func restoreAction(t *testing.T, pool *pgxpool.Pool, chainID string, seq int) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE guliangiam.records SET action = $1 WHERE chain_id = $2 AND chain_seq = $3`,
		actionFor(platformcrypto.ChainAlgoSM3, seq), chainID, seq)
	require.NoError(t, err)
}

func insertChain(t *testing.T, pool *pgxpool.Pool, orgID, chainID, algo string, entries int) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	var prev []byte
	for seq := 1; seq <= entries; seq++ {
		rec := audit.ChainRecord{
			OrganizationID: orgID,
			Action:         actionFor(algo, seq),
			ResourceType:   "audit",
			ResourceID:     fmt.Sprintf("resource-%d", seq),
			UserAgent:      "audit-verify-integration",
			Surface:        "system",
			Metadata:       map[string]any{"seed": "integration"},
			CreatedAt:      base.Add(time.Duration(seq) * time.Minute),
			ChainSeq:       int64(seq),
			PrevHash:       prev,
			ChainAlgo:      algo,
		}
		rec.EntryHash = fixtureEntryHash(t, rec)
		_, err := pool.Exec(ctx, insertAuditRecordSQL,
			uuid.NewString(), orgID, rec.Action, rec.ResourceType, rec.ResourceID,
			rec.UserAgent, rec.Surface, rec.Metadata, rec.CreatedAt,
			chainID, rec.ChainSeq, rec.PrevHash, rec.EntryHash, algo,
		)
		require.NoError(t, err)
		prev = rec.EntryHash
	}
}

// fixtureEntryHash reproduces the platform verifier's canonical payload
// (internal/platform/outbox/providers/audit/chain.go::computeEntryHash, which is
// unexported and out of this batch's editable scope). Drift between the two
// shows up as case A failing, not as a silently weaker fixture.
func fixtureEntryHash(t *testing.T, rec audit.ChainRecord) []byte {
	t.Helper()
	var sb strings.Builder
	write := func(key, value string) {
		sb.WriteString(key)
		sb.WriteByte('=')
		sb.WriteString(value)
		sb.WriteByte('\n')
	}
	write("organization_id", rec.OrganizationID)
	write("actor_id", rec.ActorID)
	write("action", rec.Action)
	write("resource_type", rec.ResourceType)
	write("resource_id", rec.ResourceID)
	write("ip", rec.IP)
	write("user_agent", rec.UserAgent)
	write("surface", rec.Surface)
	write("request_id", rec.RequestID)
	write("trace_id", rec.TraceID)
	write("metadata", fixtureCanonicalMetadata(rec.Metadata))
	write("created_at", rec.CreatedAt.UTC().Format(time.RFC3339Nano))
	// prev_hash is the trailing field and, like the production builder, has no
	// newline after it.
	sb.WriteString("prev_hash=")
	sb.WriteString(hex.EncodeToString(rec.PrevHash))

	hash, err := platformcrypto.HashByChainAlgorithm(rec.ChainAlgo, []byte(sb.String()))
	require.NoError(t, err)
	return hash
}

func fixtureCanonicalMetadata(metadata map[string]any) string {
	if len(metadata) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		fmt.Fprintf(&sb, "%v", metadata[k])
	}
	sb.WriteByte('}')
	return sb.String()
}
