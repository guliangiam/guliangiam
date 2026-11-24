package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

const stubListEntriesBatchSQL = "stub list entries batch"

type verifyBatchCall struct {
	chainID  string
	afterSeq int64
	limit    int
}

// chainRecordRows serves full ChainRecord rows for the batch verifier tests.
type chainRecordRows struct {
	records []ChainRecord
	idx     int
}

func (r *chainRecordRows) Close()                                       {}
func (r *chainRecordRows) Err() error                                   { return nil }
func (r *chainRecordRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *chainRecordRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *chainRecordRows) TypeMap() *pgtype.Map                         { return nil }
func (r *chainRecordRows) Conn() *pgx.Conn                              { return nil }
func (r *chainRecordRows) RawValues() [][]byte                          { return nil }

func (r *chainRecordRows) Next() bool {
	if r.idx >= len(r.records) {
		return false
	}
	r.idx++
	return true
}

func (r *chainRecordRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.records) {
		return errors.New("no current row")
	}
	rec := r.records[r.idx-1]
	meta, err := json.Marshal(rec.Metadata)
	if err != nil {
		return err
	}
	*dest[0].(*string) = rec.OrganizationID
	*dest[1].(*string) = rec.ActorID
	*dest[2].(*string) = rec.Action
	*dest[3].(*string) = rec.ResourceType
	*dest[4].(*string) = rec.ResourceID
	*dest[5].(*string) = rec.IP
	*dest[6].(*string) = rec.UserAgent
	*dest[7].(*string) = rec.Surface
	*dest[8].(*string) = rec.RequestID
	*dest[9].(*string) = rec.TraceID
	*dest[10].(*[]byte) = meta
	*dest[11].(*time.Time) = rec.CreatedAt
	*dest[12].(*int64) = rec.ChainSeq
	*dest[13].(*[]byte) = rec.PrevHash
	*dest[14].(*[]byte) = rec.EntryHash
	*dest[15].(*string) = rec.ChainAlgo
	return nil
}

func (r *chainRecordRows) Values() ([]any, error) {
	if r.idx == 0 || r.idx > len(r.records) {
		return nil, errors.New("no current row")
	}
	rec := r.records[r.idx-1]
	return []any{
		rec.OrganizationID, rec.ActorID, rec.Action, rec.ResourceType, rec.ResourceID,
		rec.IP, rec.UserAgent, rec.Surface, rec.RequestID, rec.TraceID,
		rec.Metadata, rec.CreatedAt, rec.ChainSeq, rec.PrevHash, rec.EntryHash, rec.ChainAlgo,
	}, nil
}

// verifyStubDB serves the worker's two reads: active chain IDs and bounded
// full-record pages.
type verifyStubDB struct {
	chains        []string
	records       map[string][]ChainRecord
	listChainsErr error
	queryErr      map[string]error
	blockedChain  string

	mu    sync.Mutex
	calls []verifyBatchCall
}

func (f *verifyStubDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (f *verifyStubDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	switch sql {
	case stubListChainsSQL:
		if f.listChainsErr != nil {
			return nil, f.listChainsErr
		}
		return &chainIDRows{chains: f.chains}, nil
	case stubListEntriesBatchSQL:
		chainID := args[0].(string)
		afterSeq := args[1].(int64)
		limit := args[2].(int)
		f.mu.Lock()
		f.calls = append(f.calls, verifyBatchCall{chainID: chainID, afterSeq: afterSeq, limit: limit})
		f.mu.Unlock()

		if err := f.queryErr[chainID]; err != nil {
			return nil, err
		}
		if f.blockedChain == chainID {
			<-ctx.Done()
			return nil, ctx.Err()
		}

		all := f.records[chainID]
		page := make([]ChainRecord, 0, limit)
		for _, rec := range all {
			if rec.ChainSeq <= afterSeq {
				continue
			}
			page = append(page, rec)
			if len(page) == limit {
				break
			}
		}
		return &chainRecordRows{records: page}, nil
	default:
		return nil, errors.New("unexpected Query: " + sql)
	}
}

func (f *verifyStubDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return scanFuncRow{fn: func(...any) error { return errors.New("unexpected QueryRow") }}
}

func (f *verifyStubDB) batchCalls() []verifyBatchCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]verifyBatchCall(nil), f.calls...)
}

func newVerifyTestWorker(db *verifyStubDB, batchSize int, out io.Writer) *ChainWorker {
	writer := NewAuditChainWriter(db, ChainSQL{
		ListActiveChains: stubListChainsSQL,
		ListEntriesBatch: stubListEntriesBatchSQL,
	})
	worker := NewChainWorker(writer, time.Minute, time.Minute,
		slog.New(slog.NewJSONHandler(out, &slog.HandlerOptions{Level: slog.LevelDebug})))
	worker.verifyBatchSize = batchSize
	worker.verifyBatchPause = 0
	worker.verifyTimeout = 5 * time.Second
	return worker
}

// TestChainWorker_VerifyChainFull_MatchesVerifyChainRecordsAcrossBatches pins
// the contract the CLI relies on: for the same chain, the batched worker and
// cmd/audit-verify (which calls VerifyChainRecords) return the same verified
// count, the same diagnostic text, and the same breakpoint chain_seq.
func TestChainWorker_VerifyChainFull_MatchesVerifyChainRecordsAcrossBatches(t *testing.T) {
	tests := []struct {
		name      string
		length    int
		batchSize int
		tamperSeq int
	}{
		{name: "single record", length: 1, batchSize: 2},
		{name: "exact batch boundary", length: 2, batchSize: 2},
		{name: "boundary plus one", length: 3, batchSize: 2},
		{name: "several batches", length: 7, batchSize: 2},
		{name: "tamper before first boundary", length: 5, batchSize: 2, tamperSeq: 1},
		{name: "tamper in second batch", length: 5, batchSize: 2, tamperSeq: 3},
		{name: "tamper at exact boundary tail", length: 4, batchSize: 2, tamperSeq: 3},
		{name: "tamper last entry several batches", length: 6, batchSize: 3, tamperSeq: 6},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			records := buildChain(t, tc.length, platformcrypto.ChainAlgoSHA256)
			if tc.tamperSeq > 0 {
				records[tc.tamperSeq-1].Action = "audit.tampered"
			}

			fullCount, fullErr := VerifyChainRecords("org-1", records)

			db := &verifyStubDB{
				chains:  []string{"org-1"},
				records: map[string][]ChainRecord{"org-1": records},
			}
			worker := newVerifyTestWorker(db, tc.batchSize, io.Discard)
			result, workerErr := worker.verifyChainFull(context.Background(), "org-1")

			assert.Equal(t, fullCount, result.Verified, "batched verification must count the same valid prefix as the CLI path")
			if fullErr == nil {
				require.NoError(t, workerErr)
				assert.Equal(t, int64(0), result.BreakSeq)
				assert.Empty(t, result.Stage)
				assert.Equal(t, tc.length, result.Verified)
			} else {
				require.Error(t, workerErr)
				assert.Equal(t, fullErr.Error(), workerErr.Error(), "the CLI and worker must emit the same diagnostic text")
				assert.Equal(t, int64(tc.tamperSeq), result.BreakSeq, "the worker must report the exact chain_seq breakpoint")
				assert.Equal(t, chainVerifyStageVerify, result.Stage)
			}

			calls := db.batchCalls()
			require.NotEmpty(t, calls)
			assert.Equal(t, int64(0), calls[0].afterSeq)
			assert.Equal(t, tc.batchSize, calls[0].limit)
			for i := 1; i < len(calls); i++ {
				assert.Greater(t, calls[i].afterSeq, calls[i-1].afterSeq, "pages must advance by a strict keyset cursor")
			}
		})
	}
}

// TestChainWorker_DetectsFieldTamperWithIntactLinks is the R1 regression: the
// row's prev_hash still points at the previous entry_hash, so a link-only check
// passes; only recomputing entry_hash from the fields can locate the tamper.
func TestChainWorker_DetectsFieldTamperWithIntactLinks(t *testing.T) {
	records := buildChain(t, 5, platformcrypto.ChainAlgoSHA256)
	records[2].UserAgent = "tampered-agent"
	require.Equal(t, records[2].EntryHash, records[3].PrevHash,
		"the link relationship must stay intact for this to be an R1 regression")

	var logBuf bytes.Buffer
	db := &verifyStubDB{
		chains:  []string{"org-1"},
		records: map[string][]ChainRecord{"org-1": records},
	}
	worker := newVerifyTestWorker(db, 2, &logBuf)

	worker.verifyAll(context.Background())

	logs := logBuf.String()
	assert.Contains(t, logs, `"msg":"audit chain integrity FAILED"`)
	assert.Contains(t, logs, `"chain_id":"org-1"`)
	assert.Contains(t, logs, `"chain_seq":3`)
	assert.Contains(t, logs, `"entries_verified":2`)
	assert.Contains(t, logs, "entry_hash mismatch")
}

func TestChainWorker_VerifyAll_AcceptsIntactSHA256AndSM3Chains(t *testing.T) {
	shaRecords := buildChain(t, 3, platformcrypto.ChainAlgoSHA256)
	sm3Records := buildChain(t, 4, platformcrypto.ChainAlgoSM3)

	var logBuf bytes.Buffer
	db := &verifyStubDB{
		chains: []string{"sha-chain", "sm3-chain"},
		records: map[string][]ChainRecord{
			"sha-chain": shaRecords,
			"sm3-chain": sm3Records,
		},
	}
	worker := newVerifyTestWorker(db, 2, &logBuf)

	worker.verifyAll(context.Background())

	logs := logBuf.String()
	assert.NotContains(t, logs, `"level":"ERROR"`)
	assert.Contains(t, logs, `"chain_id":"sha-chain","entries":3`)
	assert.Contains(t, logs, `"chain_id":"sm3-chain","entries":4`)
}

func TestChainWorker_RejectsAlgorithmSwitchAcrossBatchBoundary(t *testing.T) {
	records := buildChain(t, 4, platformcrypto.ChainAlgoSHA256)
	records[2].ChainAlgo = platformcrypto.ChainAlgoSM3

	db := &verifyStubDB{
		chains:  []string{"org-1"},
		records: map[string][]ChainRecord{"org-1": records},
	}
	worker := newVerifyTestWorker(db, 2, io.Discard)

	verified, workerErr := worker.verifyChainFull(context.Background(), "org-1")
	require.Error(t, workerErr)
	assert.Equal(t, 2, verified.Verified)
	assert.Equal(t, int64(3), verified.BreakSeq)
	assert.Equal(t, chainVerifyStageVerify, verified.Stage)
	assert.Contains(t, workerErr.Error(), "chain_algo changed")
	assert.Contains(t, workerErr.Error(), platformcrypto.ChainAlgoSHA256)
	assert.Contains(t, workerErr.Error(), platformcrypto.ChainAlgoSM3)
}

func TestChainWorker_DetectsBrokenLinkAtBatchBoundary(t *testing.T) {
	records := buildChain(t, 4, platformcrypto.ChainAlgoSHA256)
	records[2].PrevHash = []byte{0x00, 0x01}

	db := &verifyStubDB{
		chains:  []string{"org-1"},
		records: map[string][]ChainRecord{"org-1": records},
	}
	worker := newVerifyTestWorker(db, 2, io.Discard)

	result, err := worker.verifyChainFull(context.Background(), "org-1")
	require.Error(t, err)
	assert.Equal(t, 2, result.Verified)
	assert.Equal(t, int64(3), result.BreakSeq)
	assert.Equal(t, chainVerifyStageVerify, result.Stage)
	assert.Contains(t, err.Error(), "prev_hash mismatch (chain broken)")
}

func TestChainWorker_VerifyAll_LoadFailureContinuesWithNextChain(t *testing.T) {
	var logBuf bytes.Buffer
	db := &verifyStubDB{
		chains: []string{"broken-load", "healthy"},
		records: map[string][]ChainRecord{
			"healthy": buildChain(t, 2, platformcrypto.ChainAlgoSM3),
		},
		queryErr: map[string]error{"broken-load": errors.New("simulated load failure")},
	}
	worker := newVerifyTestWorker(db, 2, &logBuf)

	worker.verifyAll(context.Background())

	logs := logBuf.String()
	assert.Contains(t, logs, `"msg":"audit chain verification failed"`)
	assert.Contains(t, logs, `"stage":"load"`)
	assert.Contains(t, logs, `"chain_id":"broken-load"`)
	assert.Contains(t, logs, "load batch after seq 0")
	assert.Contains(t, logs, `"chain_id":"healthy","entries":2`, "a failed chain must not stop verification of the next chain")
}

func TestChainWorker_VerifyAll_TimeoutContinuesWithNextChain(t *testing.T) {
	var logBuf bytes.Buffer
	db := &verifyStubDB{
		chains:       []string{"slow", "healthy"},
		records:      map[string][]ChainRecord{"healthy": buildChain(t, 2, platformcrypto.ChainAlgoSHA256)},
		blockedChain: "slow",
	}
	worker := newVerifyTestWorker(db, 2, &logBuf)
	worker.verifyTimeout = 20 * time.Millisecond

	worker.verifyAll(context.Background())

	logs := logBuf.String()
	assert.Contains(t, logs, `"msg":"audit chain verification timed out"`)
	assert.Contains(t, logs, `"stage":"load"`)
	assert.Contains(t, logs, `"chain_id":"slow"`)
	assert.Contains(t, logs, `"chain_id":"healthy","entries":2`, "a timeout must be scoped to the chain, not the whole cycle")
}

func TestChainWorker_VerifyAll_ObservesExistingDurationSeries(t *testing.T) {
	healthy := buildChain(t, 2, platformcrypto.ChainAlgoSM3)
	tampered := buildChain(t, 3, platformcrypto.ChainAlgoSHA256)
	tampered[1].Action = "audit.tampered"

	db := &verifyStubDB{
		chains: []string{"tampered", "healthy"},
		records: map[string][]ChainRecord{
			"tampered": tampered,
			"healthy":  healthy,
		},
	}
	worker := newVerifyTestWorker(db, 2, io.Discard)

	beforeOK := auditChainBuildSamples(t, metricResultOK)
	beforeError := auditChainBuildSamples(t, metricResultError)
	worker.verifyAll(context.Background())

	assert.Equal(t, beforeOK+1, auditChainBuildSamples(t, metricResultOK))
	assert.Equal(t, beforeError+1, auditChainBuildSamples(t, metricResultError))
}

func TestListChainRecordsBatch_PassesCursorAndDecodesFullFields(t *testing.T) {
	records := buildChain(t, 3, platformcrypto.ChainAlgoSM3)
	db := &verifyStubDB{records: map[string][]ChainRecord{"org-1": records}}
	writer := NewAuditChainWriter(db, ChainSQL{ListEntriesBatch: stubListEntriesBatchSQL})

	batch, err := writer.ListChainRecordsBatch(context.Background(), "org-1", 1, 2)
	require.NoError(t, err)
	require.Len(t, batch, 2)
	assert.Equal(t, int64(2), batch[0].ChainSeq)
	assert.Equal(t, int64(3), batch[1].ChainSeq)
	assert.Equal(t, "org-1", batch[0].OrganizationID)
	assert.Equal(t, records[1].CreatedAt, batch[0].CreatedAt)
	assert.Equal(t, records[1].PrevHash, batch[0].PrevHash)
	assert.Equal(t, records[1].EntryHash, batch[0].EntryHash)
	assert.Equal(t, platformcrypto.ChainAlgoSM3, batch[0].ChainAlgo)
	assert.Equal(t, float64(1), batch[0].Metadata["index"], "metadata must be decoded for hash recomputation")

	calls := db.batchCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, verifyBatchCall{chainID: "org-1", afterSeq: 1, limit: 2}, calls[0])

	_, err = writer.ListChainRecordsBatch(context.Background(), "org-1", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch limit must be positive")
}
