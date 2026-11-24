package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func buildChain(t *testing.T, n int, algo string) []ChainRecord {
	t.Helper()
	base := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	var out []ChainRecord
	var prev []byte
	for i := 0; i < n; i++ {
		rec := ChainRecord{
			OrganizationID: "org-1",
			ActorID:        "user-1",
			Action:         "user.updated",
			ResourceType:   "user",
			ResourceID:     "user-1",
			Surface:        "management",
			Metadata:       map[string]any{"index": float64(i)},
			CreatedAt:      base.Add(time.Duration(i) * time.Second),
			ChainSeq:       int64(i + 1),
			PrevHash:       prev,
			ChainAlgo:      algo,
		}
		hash, err := computeEntryHash(
			algo,
			rec.OrganizationID, rec.ActorID, rec.Action, rec.ResourceType, rec.ResourceID,
			rec.IP, rec.UserAgent, rec.Surface, rec.RequestID, rec.TraceID,
			rec.Metadata, rec.CreatedAt, rec.PrevHash,
		)
		require.NoError(t, err)
		rec.EntryHash = hash
		prev = rec.EntryHash
		out = append(out, rec)
	}
	return out
}

func TestVerifyChainRecords_SHA256OK(t *testing.T) {
	records := buildChain(t, 3, platformcrypto.ChainAlgoSHA256)
	count, err := VerifyChainRecords("org-1", records)
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestVerifyChainRecords_SM3OK(t *testing.T) {
	records := buildChain(t, 3, platformcrypto.ChainAlgoSM3)
	count, err := VerifyChainRecords("org-1", records)
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestVerifyChainRecords_DetectsTamperedEntry(t *testing.T) {
	records := buildChain(t, 3, platformcrypto.ChainAlgoSHA256)
	records[1].Action = "user.deleted" // tamper without recomputing entry_hash

	count, err := VerifyChainRecords("org-1", records)
	assert.Error(t, err)
	assert.Equal(t, 1, count)
	assert.Contains(t, err.Error(), "seq 2")
	assert.Contains(t, err.Error(), "sha256")
}

func TestVerifyChainRecords_SM3TamperLocatesSeq(t *testing.T) {
	records := buildChain(t, 4, platformcrypto.ChainAlgoSM3)
	records[2].Action = "user.deleted"

	count, err := VerifyChainRecords("org-1", records)
	require.Error(t, err)
	assert.Equal(t, 2, count)
	assert.Contains(t, err.Error(), "seq 3")
	assert.Contains(t, err.Error(), "sm3")
}

func TestVerifyChainRecords_DetectsBrokenPrevHash(t *testing.T) {
	records := buildChain(t, 3, platformcrypto.ChainAlgoSHA256)
	records[2].PrevHash = []byte{0x00, 0x01}

	count, err := VerifyChainRecords("org-1", records)
	assert.Error(t, err)
	assert.Equal(t, 2, count)
	assert.Contains(t, err.Error(), "prev_hash")
}

func TestVerifyChainRecords_RejectsMixedAlgorithms(t *testing.T) {
	records := buildChain(t, 3, platformcrypto.ChainAlgoSHA256)
	records[1].ChainAlgo = platformcrypto.ChainAlgoSM3

	count, err := VerifyChainRecords("org-1", records)
	require.Error(t, err)
	assert.Equal(t, 1, count)
	assert.Contains(t, err.Error(), "chain_algo changed")
	assert.Contains(t, err.Error(), "sha256")
	assert.Contains(t, err.Error(), "sm3")
	assert.Contains(t, err.Error(), "seq 2")
}

func TestVerifyChainRecords_RejectsUnknownAlgorithm(t *testing.T) {
	records := buildChain(t, 2, platformcrypto.ChainAlgoSHA256)
	records[0].ChainAlgo = "md5"

	_, err := VerifyChainRecords("org-1", records)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported audit chain algorithm")
	assert.Contains(t, err.Error(), "md5")
	assert.Contains(t, err.Error(), "seq 1")
}

func TestVerifyChainRecords_EmptyChainAlgoIsSHA256Compatibility(t *testing.T) {
	records := buildChain(t, 3, platformcrypto.ChainAlgoSHA256)
	for i := range records {
		records[i].ChainAlgo = ""
	}
	count, err := VerifyChainRecords("org-1", records)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

// ── Compute: empty chain default / non-empty chain inheritance ──────────────

type lastEntryRow struct {
	seq  int64
	hash []byte
	algo string
	err  error
}

func (r lastEntryRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*int64)) = r.seq
	*(dest[1].(*[]byte)) = r.hash
	*(dest[2].(*string)) = r.algo
	return nil
}

type fakeChainDB struct {
	row lastEntryRow
}

func (fakeChainDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (fakeChainDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (f fakeChainDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return f.row
}

func computeTestEntry(t *testing.T, db fakeChainDB) *ChainEntry {
	t.Helper()
	writer := NewAuditChainWriter(db, ChainSQL{})
	entry, err := writer.Compute(context.Background(),
		"org-1", "", "user.updated", "user", "user-1",
		"", "", "management", "", "",
		map[string]any{}, time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	return entry
}

func TestCompute_EmptyChainUsesProfileDefault(t *testing.T) {
	t.Setenv("CRYPTO_PROFILE", "gm")
	entry := computeTestEntry(t, fakeChainDB{row: lastEntryRow{err: pgx.ErrNoRows}})
	assert.Equal(t, int64(1), entry.ChainSeq)
	assert.Equal(t, platformcrypto.ChainAlgoSM3, entry.ChainAlgo)
	assert.Nil(t, entry.PrevHash)
}

func TestCompute_NonEmptyChainInheritsTailAlgorithm(t *testing.T) {
	t.Setenv("CRYPTO_PROFILE", "default")
	tailHash := []byte{0x01, 0x02}
	entry := computeTestEntry(t, fakeChainDB{row: lastEntryRow{seq: 5, hash: tailHash, algo: platformcrypto.ChainAlgoSM3}})
	assert.Equal(t, int64(6), entry.ChainSeq)
	assert.Equal(t, platformcrypto.ChainAlgoSM3, entry.ChainAlgo, "the tail algorithm must win over the profile default")
	assert.Equal(t, tailHash, entry.PrevHash)
}

func TestCompute_LegacyEmptyTailAlgorithmIsSHA256(t *testing.T) {
	t.Setenv("CRYPTO_PROFILE", "gm")
	entry := computeTestEntry(t, fakeChainDB{row: lastEntryRow{seq: 1, hash: []byte{0x01}, algo: ""}})
	assert.Equal(t, platformcrypto.ChainAlgoSHA256, entry.ChainAlgo)
}

// 写入侧 time.Now() 带纳秒，而 records.created_at 仅存微秒；若哈希不归一，
// 读回重算必然不一致，所有链都会被误判 tampered（S-17 / DEF-47）。
func TestComputeEntryHash_NormalizesSubMicrosecondTime(t *testing.T) {
	base := time.Date(2026, 10, 19, 10, 0, 0, 123456000, time.UTC)
	hash := func(at time.Time) []byte {
		t.Helper()
		out, err := computeEntryHash(
			platformcrypto.ChainAlgoSHA256,
			"org-1", "user-1", "user.updated", "user", "user-1",
			"", "", "management", "", "",
			map[string]any{}, at, nil,
		)
		require.NoError(t, err)
		return out
	}
	assert.Equal(t, hash(base), hash(base.Add(789*time.Nanosecond)))
}
