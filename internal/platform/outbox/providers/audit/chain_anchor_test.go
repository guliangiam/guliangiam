package audit

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// testChainID doubles as organization_id: the worker anchors ChainWriter chains
// by organization ID, and Anchor only needs the organization to be a UUID when
// a new anchor row is inserted.
const testChainID = "01a0ea76-0c00-7000-8000-000000000001"

// Golden digests of "chain_id=<testChainID>\nanchor_seq=7\nentry_hash=<32×01>".
// They pin the offline recipe documented on Anchor/computeAnchorProof; a change
// to the canonical form or digest must update these constants deliberately
// (an offline verifier built on the old recipe would stop agreeing).
const (
	goldenSHA256Proof = "sha256:bee3fb5ca2b8123f4d5d61661f6cef4ea888358927f714bcca4a76dfb701dd39"
	goldenSM3Proof    = "sm3:cfccfc2683c7a31c39ad01d4a9e3c35f51a8bdfbc49ae5aed7703519d68ef4c8"
)

func TestComputeAnchorProof_DeterministicRecipe(t *testing.T) {
	entryHash := bytes.Repeat([]byte{0x01}, 32)

	proof, err := computeAnchorProof(platformcrypto.ChainAlgoSHA256, testChainID, 7, entryHash)
	require.NoError(t, err)
	assert.Equal(t, goldenSHA256Proof, proof)
	assert.LessOrEqual(t, len(proof), 255)

	// Recompute from the documented canonical string, independently of the
	// builder inside computeAnchorProof.
	canonical := "chain_id=" + testChainID + "\nanchor_seq=7\nentry_hash=" + strings.Repeat("01", 32)
	digest, err := platformcrypto.HashByChainAlgorithm(platformcrypto.ChainAlgoSHA256, []byte(canonical))
	require.NoError(t, err)
	assert.Equal(t, "sha256:"+hex.EncodeToString(digest), proof)

	again, err := computeAnchorProof(platformcrypto.ChainAlgoSHA256, testChainID, 7, entryHash)
	require.NoError(t, err)
	assert.Equal(t, proof, again, "same input must produce the same proof")
}

func TestComputeAnchorProof_SM3AndAlgorithmPrefix(t *testing.T) {
	entryHash := bytes.Repeat([]byte{0x01}, 32)

	sm3Proof, err := computeAnchorProof(platformcrypto.ChainAlgoSM3, testChainID, 7, entryHash)
	require.NoError(t, err)
	assert.Equal(t, goldenSM3Proof, sm3Proof)
	assert.True(t, strings.HasPrefix(sm3Proof, "sm3:"))
	assert.NotEqual(t, goldenSHA256Proof, sm3Proof, "the digest must follow the declared algorithm")
}

func TestComputeAnchorProof_LegacyEmptyAlgoIsSHA256(t *testing.T) {
	entryHash := bytes.Repeat([]byte{0x01}, 32)

	proof, err := computeAnchorProof("", testChainID, 7, entryHash)
	require.NoError(t, err)
	assert.Equal(t, goldenSHA256Proof, proof, "empty legacy chain_algo keeps the SHA-256 compatibility rule")
}

func TestComputeAnchorProof_RejectsEmptyEntryHash(t *testing.T) {
	_, err := computeAnchorProof(platformcrypto.ChainAlgoSHA256, testChainID, 7, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry_hash is empty")

	_, err = computeAnchorProof(platformcrypto.ChainAlgoSHA256, testChainID, 7, []byte{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry_hash is empty")
}

func TestComputeAnchorProof_RejectsUnknownAlgorithm(t *testing.T) {
	_, err := computeAnchorProof("md5", testChainID, 7, bytes.Repeat([]byte{0x01}, 32))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported audit chain algorithm")
}

// ── Anchor behaviour ────────────────────────────────────────────────────────

const (
	stubAdvisoryLockSQL    = "stub advisory lock"
	stubLoadLastEntrySQL   = "stub load last entry"
	stubLoadAnchorSeqSQL   = "stub load anchor seq"
	stubInsertAnchorSQL    = "stub insert anchor and advance"
	stubAdvanceAnchoredSQL = "stub advance anchored"
)

type anchorExecCall struct {
	sql  string
	args []any
}

type anchorStubDB struct {
	tailSeq      int64
	tailHash     []byte
	tailAlgo     string
	maxAnchorSeq int64
	execs        []anchorExecCall
}

func (f *anchorStubDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execs = append(f.execs, anchorExecCall{sql: sql, args: args})
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (f *anchorStubDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (f *anchorStubDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch sql {
	case stubLoadLastEntrySQL:
		return scanFuncRow{fn: func(dest ...any) error {
			*(dest[0].(*int64)) = f.tailSeq
			*(dest[1].(*[]byte)) = f.tailHash
			*(dest[2].(*string)) = f.tailAlgo
			return nil
		}}
	case stubLoadAnchorSeqSQL:
		return scanFuncRow{fn: func(dest ...any) error {
			*(dest[0].(*int64)) = f.maxAnchorSeq
			return nil
		}}
	default:
		return scanFuncRow{fn: func(...any) error { return errors.New("unexpected QueryRow: " + sql) }}
	}
}

type scanFuncRow struct {
	fn func(dest ...any) error
}

func (r scanFuncRow) Scan(dest ...any) error { return r.fn(dest...) }

func newAnchorTestWriter(db *anchorStubDB) *AuditChainWriter {
	return NewAuditChainWriter(db, ChainSQL{
		AdvisoryLock:           stubAdvisoryLockSQL,
		LoadLastEntry:          stubLoadLastEntrySQL,
		LoadAnchorSeq:          stubLoadAnchorSeqSQL,
		InsertAnchorAndAdvance: stubInsertAnchorSQL,
		AdvanceChainedStatus:   stubAdvanceAnchoredSQL,
	})
}

func execsMatching(db *anchorStubDB, sql string) []anchorExecCall {
	var out []anchorExecCall
	for _, call := range db.execs {
		if call.sql == sql {
			out = append(out, call)
		}
	}
	return out
}

func TestAnchor_NewTailInsertsAnchorAndAdvancesStatus(t *testing.T) {
	tailHash := bytes.Repeat([]byte{0x01}, 32)
	db := &anchorStubDB{tailSeq: 7, tailHash: tailHash, tailAlgo: platformcrypto.ChainAlgoSHA256, maxAnchorSeq: 3}
	writer := newAnchorTestWriter(db)

	require.NoError(t, writer.Anchor(context.Background(), testChainID, testChainID, ""))

	inserts := execsMatching(db, stubInsertAnchorSQL)
	require.Len(t, inserts, 1, "a new tail must insert exactly one anchor")
	require.Len(t, inserts[0].args, 6)
	assert.Equal(t, testChainID, inserts[0].args[2])
	assert.Equal(t, int64(7), inserts[0].args[3], "anchor_seq must be the covered tail chain_seq, not an ordinal")
	assert.Equal(t, tailHash, inserts[0].args[4])
	assert.Equal(t, goldenSHA256Proof, inserts[0].args[5])
	assert.Empty(t, execsMatching(db, stubAdvanceAnchoredSQL), "the insert carries the status transition in the same statement")
}

func TestAnchor_TailAlreadyAnchoredBackfillsWithoutInsert(t *testing.T) {
	tailHash := bytes.Repeat([]byte{0x01}, 32)
	db := &anchorStubDB{tailSeq: 7, tailHash: tailHash, tailAlgo: platformcrypto.ChainAlgoSHA256, maxAnchorSeq: 7}
	writer := newAnchorTestWriter(db)

	require.NoError(t, writer.Anchor(context.Background(), testChainID, testChainID, ""))

	assert.Empty(t, execsMatching(db, stubInsertAnchorSQL), "an unchanged tail must not be anchored again every worker tick")
	backfills := execsMatching(db, stubAdvanceAnchoredSQL)
	require.Len(t, backfills, 1)
	require.Len(t, backfills[0].args, 2)
	assert.Equal(t, testChainID, backfills[0].args[0])
	assert.Equal(t, int64(7), backfills[0].args[1], "the backfill must cover the existing anchor range")
}

func TestAnchor_ExternalProofIsStoredVerbatim(t *testing.T) {
	const externalProof = "tsa:receipt-abc"
	db := &anchorStubDB{
		tailSeq: 7, tailHash: bytes.Repeat([]byte{0x01}, 32),
		tailAlgo: platformcrypto.ChainAlgoSHA256, maxAnchorSeq: 3,
	}
	writer := newAnchorTestWriter(db)

	require.NoError(t, writer.Anchor(context.Background(), testChainID, testChainID, externalProof))
	inserts := execsMatching(db, stubInsertAnchorSQL)
	require.Len(t, inserts, 1)
	assert.Equal(t, externalProof, inserts[0].args[5], "a non-empty proof is an externally injected receipt")
}

func TestAnchor_RejectsOverlongExternalProof(t *testing.T) {
	db := &anchorStubDB{
		tailSeq: 7, tailHash: bytes.Repeat([]byte{0x01}, 32),
		tailAlgo: platformcrypto.ChainAlgoSHA256, maxAnchorSeq: 3,
	}
	writer := newAnchorTestWriter(db)

	err := writer.Anchor(context.Background(), testChainID, testChainID, strings.Repeat("x", 256))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 255 bytes")
	assert.Empty(t, execsMatching(db, stubInsertAnchorSQL))
}

func TestAnchor_EmptyChainIsRejected(t *testing.T) {
	db := &anchorStubDB{tailAlgo: platformcrypto.ChainAlgoSHA256}
	writer := newAnchorTestWriter(db)

	err := writer.Anchor(context.Background(), testChainID, testChainID, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing to anchor")
	assert.Empty(t, execsMatching(db, stubInsertAnchorSQL))
	assert.Empty(t, execsMatching(db, stubAdvanceAnchoredSQL))
}
