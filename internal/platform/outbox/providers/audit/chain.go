package audit

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5/pgconn"
)

// ChainEntry holds the chain fields computed for an audit record.
type ChainEntry struct {
	ChainID   string
	ChainSeq  int64
	PrevHash  []byte
	EntryHash []byte
	ChainAlgo string
}

// dbtx 是本包消费的最小数据库端口（transport/app 可传入事务或连接池）。
// 在包内声明而不是 import infra/postgres/shared：platform 不得反向依赖 infra
// （分层门禁 Rule 7），且端口只包含链计算需要的三个方法。
type dbtx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// AuditChainWriter computes hash-chain linkage fields for audit records.
//
// Each organization has a single chain identified by its organization ID.
// For every new record the writer:
//  1. Loads the last entry in the org's chain (by chain_seq DESC).
//  2. Sets chain_seq = last.chain_seq + 1 (0 for the first record).
//  3. Sets prev_hash = last.entry_hash (nil for the first record).
//  4. Computes entry_hash with the chain's algorithm (sha256/sm3): the first
//     entry follows CRYPTO_PROFILE, later entries inherit the tail chain_algo.
//
// Compute must be called within the same transaction as the record INSERT so
// that chain_seq assignment is atomic and gap-free.
//
// 持有最小 DB 端口 dbtx（由调用方传入）与 SQL provider；SQL 文本由
// infra/postgres/records 通过组合根注入，platform/domain 不再承载 SQL 常量。
type AuditChainWriter struct {
	db  dbtx
	sql ChainSQL
}

// NewAuditChainWriter creates a writer backed by the given DBTX and SQL provider.
func NewAuditChainWriter(db dbtx, sql ChainSQL) *AuditChainWriter {
	return &AuditChainWriter{db: db, sql: sql}
}

// Compute computes the chain linkage for a record. It queries the last chain
// entry (scoped to the current transaction) and returns the fields to store
// on the new record.
func (w *AuditChainWriter) Compute(ctx context.Context,
	organizationID, actorID, action, resourceType, resourceID,
	ip, userAgent, surface, requestID, traceID string,
	metadata map[string]any,
	createdAt time.Time,
) (*ChainEntry, error) {
	chainID := organizationID

	lastSeq, lastHash, lastAlgo, err := w.loadLastEntry(ctx, chainID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "load last chain entry")
	}

	chainSeq := lastSeq + 1
	var prevHash []byte
	if lastHash != nil {
		prevHash = lastHash
	}

	// 空链由部署档位决定首条算法；非空链无条件继承链尾算法，避免同一链
	// 中途切换算法（切换会让 prev_hash 的验证语义失去唯一口径）。
	chainAlgo := lastAlgo
	if lastHash == nil {
		chainAlgo = platformcrypto.DefaultChainAlgorithm()
	} else {
		normalized, normalizeErr := platformcrypto.NormalizeChainAlgorithm(chainAlgo)
		if normalizeErr != nil {
			return nil, apperrors.WrapInternal(normalizeErr, "compute audit chain entry hash")
		}
		chainAlgo = normalized
	}

	entryHash, err := computeEntryHash(
		chainAlgo,
		organizationID, actorID, action, resourceType, resourceID,
		ip, userAgent, surface, requestID, traceID,
		metadata, createdAt, prevHash,
	)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "compute audit chain entry hash")
	}

	return &ChainEntry{
		ChainID:   chainID,
		ChainSeq:  chainSeq,
		PrevHash:  prevHash,
		EntryHash: entryHash,
		ChainAlgo: chainAlgo,
	}, nil
}

// loadLastEntry returns the (chain_seq, entry_hash, chain_algo) of the latest
// record in the given chain, or (0, nil, "") if the chain is empty.
//
// Serialisation: acquires a per-chain advisory lock (pg_advisory_xact_lock)
// before reading so concurrent Compute() calls never observe the same
// MAX(chain_seq) and collide on the unique constraint uq_records_chain_seq.
// The lock is released automatically when the enclosing transaction ends.
func (w *AuditChainWriter) loadLastEntry(ctx context.Context, chainID string) (seqOut int64, hashOut []byte, algoOut string, err error) {
	if _, err := w.db.Exec(ctx, w.sql.AdvisoryLock, chainID); err != nil {
		return 0, nil, "", apperrors.WrapInternal(err, "acquire chain advisory lock")
	}
	var seq int64
	var hash []byte
	var algo string
	err = w.db.QueryRow(ctx, w.sql.LoadLastEntry, chainID).Scan(&seq, &hash, &algo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			return 0, nil, "", nil
		}
		return 0, nil, "", err
	}
	return seq, hash, algo, nil
}

// computeEntryHash returns the digest (selected by chain_algo) of a canonical
// serialization of the record fields concatenated with the previous entry hash.
func computeEntryHash(
	algo,
	organizationID, actorID, action, resourceType, resourceID,
	ip, userAgent, surface, requestID, traceID string,
	metadata map[string]any,
	createdAt time.Time,
	prevHash []byte,
) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("organization_id=")
	sb.WriteString(organizationID)
	sb.WriteByte('\n')
	sb.WriteString("actor_id=")
	sb.WriteString(actorID)
	sb.WriteByte('\n')
	sb.WriteString("action=")
	sb.WriteString(action)
	sb.WriteByte('\n')
	sb.WriteString("resource_type=")
	sb.WriteString(resourceType)
	sb.WriteByte('\n')
	sb.WriteString("resource_id=")
	sb.WriteString(resourceID)
	sb.WriteByte('\n')
	sb.WriteString("ip=")
	sb.WriteString(ip)
	sb.WriteByte('\n')
	sb.WriteString("user_agent=")
	sb.WriteString(userAgent)
	sb.WriteByte('\n')
	sb.WriteString("surface=")
	sb.WriteString(surface)
	sb.WriteByte('\n')
	sb.WriteString("request_id=")
	sb.WriteString(requestID)
	sb.WriteByte('\n')
	sb.WriteString("trace_id=")
	sb.WriteString(traceID)
	sb.WriteByte('\n')
	sb.WriteString("metadata=")
	sb.WriteString(canonicalMetadata(metadata))
	sb.WriteByte('\n')
	sb.WriteString("created_at=")
	// 与 records.created_at（TIMESTAMPTZ）的存储精度对齐：PostgreSQL 只保留
	// 微秒，写入时若用纳秒参与哈希，读回重算必然不一致 → 所有链都会被误判
	// 「tampered」（S-17 审计链验证假阳性）。写/验两侧共用本函数，统一截断。
	sb.WriteString(createdAt.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano))
	sb.WriteByte('\n')
	sb.WriteString("prev_hash=")
	if prevHash != nil {
		sb.WriteString(hex.EncodeToString(prevHash))
	}

	return platformcrypto.HashByChainAlgorithm(algo, []byte(sb.String()))
}

// RecomputeEntryHash exposes the canonical chain hash for maintenance tooling
// (legacy-chain repair). It is the same function the writer and verifier use,
// so a repaired chain stays verifiable forever (LEFT-R85-01).
func RecomputeEntryHash(algo string, rec ChainRecord) ([]byte, error) {
	return computeEntryHash(
		algo,
		rec.OrganizationID, rec.ActorID, rec.Action, rec.ResourceType, rec.ResourceID,
		rec.IP, rec.UserAgent, rec.Surface, rec.RequestID, rec.TraceID,
		rec.Metadata, rec.CreatedAt, rec.PrevHash,
	)
}

// canonicalMetadata serializes a metadata map as sorted key=value pairs.
func canonicalMetadata(metadata map[string]any) string {
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

// Anchor persists a chain anchor for the given chain at its current tail and
// advances every already-chained record covered by the anchor to `anchored`.
// Both writes travel in one data-modifying CTE (see ChainSQL.InsertAnchorAndAdvance),
// so there is no state where the anchor exists but its records still claim to be
// merely chained.
//
// anchor_seq is the chain position the anchor closes (the tail chain_seq), not
// an anchor ordinal: the CTE advances exactly `chain_seq <= anchor_seq`, and the
// same value makes "this tail is already anchored" checkable without reading the
// anchors' entry hashes.
//
// A non-empty proof is an externally injected receipt and is stored verbatim
// (reserved for a future RFC 3161 timestamp authority). Passing "" lets the
// writer derive a deterministic, offline-recomputable proof:
//
//	<algo>:<hex>
//
// algo is the normalised chain_algo of the anchored tail (sha256|sm3) and hex is
// HashByChainAlgorithm(algo, canonical), where canonical is the existing
// `key=value\n` serialisation (no trailing newline):
//
//	chain_id=<chainID>\nanchor_seq=<anchorSeq>\nentry_hash=<lowercase hex>
//
// An offline verifier only needs the persisted anchor row plus this recipe; the
// result is at most 71 characters (7-byte prefix + 64 hex chars), well under the
// VARCHAR(255) column bound.
//
// Anchoring is idempotent: when the worker runs again with no new entries
// (tail chain_seq <= MAX(anchor_seq)), no second anchor row is inserted; only
// the status backfill runs.
func (w *AuditChainWriter) Anchor(ctx context.Context, organizationID, chainID, proof string) error {
	tailSeq, tailHash, tailAlgo, err := w.loadLastEntry(ctx, chainID)
	if err != nil {
		return apperrors.WrapInternal(err, "load last chain entry")
	}
	if tailHash == nil {
		return apperrors.NewInvalidArgumentf("chain %s is empty, nothing to anchor", chainID)
	}

	var maxAnchorSeq int64
	if err := w.db.QueryRow(ctx, w.sql.LoadAnchorSeq, organizationID, chainID).Scan(&maxAnchorSeq); err != nil {
		return apperrors.WrapInternal(err, "load anchor seq")
	}

	// The tail is already covered by an existing anchor: backfill only.
	if tailSeq <= maxAnchorSeq {
		return w.advanceAnchored(ctx, chainID, maxAnchorSeq)
	}

	anchorSeq := tailSeq
	if proof == "" {
		proof, err = computeAnchorProof(tailAlgo, chainID, anchorSeq, tailHash)
		if err != nil {
			return err
		}
	}
	if len(proof) > 255 {
		return apperrors.NewInvalidArgumentf("anchor proof for chain %s exceeds 255 bytes", chainID)
	}

	orgUUID, err := uuid.Parse(organizationID)
	if err != nil {
		return apperrors.WrapInvalidArgumentf(err, "invalid organization id for chain anchor")
	}
	_, err = w.db.Exec(ctx, w.sql.InsertAnchorAndAdvance,
		uuid.NewString(),
		orgUUID,
		chainID, anchorSeq, tailHash, proof,
	)
	if err != nil {
		return apperrors.WrapInternal(err, "insert anchor")
	}
	return nil
}

// advanceAnchored flips records covered by an existing anchor from chained to
// anchored without inserting another anchor row.
func (w *AuditChainWriter) advanceAnchored(ctx context.Context, chainID string, anchorSeq int64) error {
	if _, err := w.db.Exec(ctx, w.sql.AdvanceChainedStatus, chainID, anchorSeq); err != nil {
		return apperrors.WrapInternal(err, "advance anchored status")
	}
	return nil
}

// computeAnchorProof derives the deterministic anchor proof described in Anchor.
// The algorithm is normalised first (empty legacy chain_algo maps to sha256, like
// every other audit-chain read path) and appears in the proof prefix, so a
// verifier can pick the digest without re-reading the records table.
func computeAnchorProof(algo, chainID string, anchorSeq int64, entryHash []byte) (string, error) {
	if len(entryHash) == 0 {
		return "", apperrors.NewInvalidArgumentf("cannot compute anchor proof for chain %s: entry_hash is empty", chainID)
	}
	normalized, err := platformcrypto.NormalizeChainAlgorithm(algo)
	if err != nil {
		return "", apperrors.WrapInternal(err, "compute anchor proof")
	}

	var sb strings.Builder
	sb.WriteString("chain_id=")
	sb.WriteString(chainID)
	sb.WriteByte('\n')
	sb.WriteString("anchor_seq=")
	sb.WriteString(strconv.FormatInt(anchorSeq, 10))
	sb.WriteByte('\n')
	sb.WriteString("entry_hash=")
	sb.WriteString(hex.EncodeToString(entryHash))

	digest, err := platformcrypto.HashByChainAlgorithm(normalized, []byte(sb.String()))
	if err != nil {
		return "", apperrors.WrapInternal(err, "compute anchor proof")
	}
	return normalized + ":" + hex.EncodeToString(digest), nil
}

// ListActiveChains returns every chain ID that has at least one computed
// entry_hash. The worker uses it to anchor/verify all chains without holding a
// direct dbtx handle.
func (w *AuditChainWriter) ListActiveChains(ctx context.Context) ([]string, error) {
	rows, err := w.db.Query(ctx, w.sql.ListActiveChains)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chains []string
	for rows.Next() {
		var chainID string
		if err := rows.Scan(&chainID); err != nil {
			return nil, err
		}
		chains = append(chains, chainID)
	}
	return chains, rows.Err()
}

// CountPending returns the number of records still waiting for chain
// construction (chain_status='pending'). It backs the pending-entries gauge;
// failures are reported to the worker, which keeps the previous gauge value
// instead of publishing a misleading zero.
func (w *AuditChainWriter) CountPending(ctx context.Context) (int64, error) {
	var pending int64
	if err := w.db.QueryRow(ctx, w.sql.CountPending).Scan(&pending); err != nil {
		return 0, apperrors.WrapInternal(err, "count pending audit chain entries")
	}
	return pending, nil
}

// VerifyChain validates the hash chain for the given chain ID.
// It returns the number of entries verified and the first error encountered.
func (w *AuditChainWriter) VerifyChain(ctx context.Context, chainID string) (int, error) {
	rows, err := w.db.Query(ctx, w.sql.ListEntries, chainID)
	if err != nil {
		return 0, apperrors.WrapInternal(err, "query chain")
	}
	defer rows.Close()

	var (
		chainSeq  int64
		prevHash  []byte
		entryHash []byte
		chainAlgo string
	)

	var count int
	var expectedPrev []byte
	var expectedAlgo string

	for rows.Next() {
		if err := rows.Scan(&chainSeq, &prevHash, &entryHash, &chainAlgo); err != nil {
			return count, apperrors.WrapInternalf(err, "scan row %d", count)
		}

		normalizedAlgo := linkCheckChainAlgo(chainAlgo)
		if expectedAlgo == "" {
			expectedAlgo = normalizedAlgo
		} else if normalizedAlgo != expectedAlgo {
			return count, apperrors.NewInternalf(
				"chain %s seq %d: chain_algo changed from %s to %s (mixing algorithms in one chain is forbidden)",
				chainID, chainSeq, expectedAlgo, normalizedAlgo)
		}

		if expectedPrev == nil {
			if prevHash != nil {
				return count, apperrors.NewInternalf("chain %s seq %d: expected nil prev_hash (first entry), got %x", chainID, chainSeq, prevHash)
			}
		} else {
			if prevHash == nil || !bytesEqual(prevHash, expectedPrev) {
				return count, apperrors.NewInternalf("chain %s seq %d: prev_hash mismatch, chain broken", chainID, chainSeq)
			}
		}

		// 重新计算 entry_hash 验证（需要完整记录，但 chainSQLListEntries 只查了链字段）。
		// 保持 VerifyChain 现有行为：只查链字段验证断裂，不重算 hash。
		expectedPrev = entryHash
		count++
	}

	if err := rows.Err(); err != nil {
		return count, err
	}
	return count, nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ChainRecord is one records row as needed for full hash-chain verification.
// It mirrors the canonical fields fed into computeEntryHash; VerifyChainFull
// recomputes entry_hash from these and compares with the stored value, so a
// single-row tampering (field edit or hash rewrite) is detectable and located.
type ChainRecord struct {
	OrganizationID string
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     string
	IP             string
	UserAgent      string
	Surface        string
	RequestID      string
	TraceID        string
	Metadata       map[string]any
	CreatedAt      time.Time
	ChainSeq       int64
	PrevHash       []byte
	EntryHash      []byte
	ChainAlgo      string
}

// linkCheckChainAlgo normalises a stored chain_algo for VerifyChain's
// mixed-algorithm comparison. VerifyChain only checks prev_hash continuity and
// never recomputes a digest, so an out-of-schema algorithm keeps its stored
// name here: turning that link check into a digest rejection would change
// VerifyChain's contract. The recomputing paths (VerifyChainRecords, Compute)
// still reject it with chain context.
func linkCheckChainAlgo(algo string) string {
	normalized, err := platformcrypto.NormalizeChainAlgorithm(algo)
	if err != nil {
		return algo
	}
	return normalized
}

// ListChainRecordsBatch loads up to limit full records with chain_seq > afterSeq
// in ascending order. It is the bounded-read seam used by the background
// verifier: a multi-million row chain is streamed page by page instead of being
// materialised in one slice. The full field set (metadata included) is required
// because entry_hash must be recomputed; link-only rows cannot detect a field
// edit that leaves prev_hash/entry_hash consistent with each other.
func (w *AuditChainWriter) ListChainRecordsBatch(ctx context.Context, chainID string, afterSeq int64, limit int) ([]ChainRecord, error) {
	if limit <= 0 {
		return nil, apperrors.NewInvalidArgumentf("audit chain batch limit must be positive")
	}
	rows, err := w.db.Query(ctx, w.sql.ListEntriesBatch, chainID, afterSeq, limit)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "query audit chain batch")
	}
	defer rows.Close()

	records := make([]ChainRecord, 0, limit)
	for rows.Next() {
		var rec ChainRecord
		var metadataJSON []byte
		if err := rows.Scan(
			&rec.OrganizationID, &rec.ActorID, &rec.Action, &rec.ResourceType, &rec.ResourceID,
			&rec.IP, &rec.UserAgent, &rec.Surface, &rec.RequestID, &rec.TraceID,
			&metadataJSON, &rec.CreatedAt, &rec.ChainSeq, &rec.PrevHash, &rec.EntryHash, &rec.ChainAlgo,
		); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan audit chain batch row %d", len(records))
		}
		rec.Metadata = map[string]any{}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &rec.Metadata); err != nil {
				return nil, apperrors.WrapInternalf(err, "decode metadata chain seq %d", rec.ChainSeq)
			}
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternal(err, "iterate audit chain batch")
	}
	return records, nil
}

// ChainVerifyState carries the already-verified prefix between bounded batches.
// The zero value means "no entries verified yet". VerifyChainRecords starts
// from it, and VerifyChainRecordBatch returns the state the next page resumes
// from, so full-chain and batched verification share one implementation.
type ChainVerifyState struct {
	// ExpectedPrev is the entry_hash of the last verified record.
	ExpectedPrev []byte
	// ExpectedAlgo is the normalized algorithm of the first verified record;
	// later pages must not switch it.
	ExpectedAlgo string
	// Started distinguishes "nothing verified yet" from an empty page.
	Started bool
}

// VerifyChainRecords verifies a fully-loaded chain. It returns the number of
// valid entries before the first failure and a descriptive error naming the
// chain_seq where the chain is broken or the stored hash no longer matches a
// recomputation. Keeping it pure makes tamper detection unit-testable without
// a database; DB access lives in cmd/audit-verify (dialect-specific SQL stays
// out of the platform layer).
func VerifyChainRecords(chainID string, chainRecords []ChainRecord) (int, error) {
	verified, _, err := VerifyChainRecordBatch(chainID, chainRecords, ChainVerifyState{})
	return verified, err
}

// VerifyChainRecordBatch verifies one page of a chain, continuing from state.
// It is the same algorithm as VerifyChainRecords: validate prev_hash continuity,
// reject a mid-chain algorithm switch, then recompute entry_hash from the
// canonical record fields and compare it with the stored value. It returns the
// number of entries valid in this page, the state the next page resumes from,
// and the first error naming the exact chain_seq where verification stopped.
func VerifyChainRecordBatch(chainID string, chainRecords []ChainRecord, state ChainVerifyState) (int, ChainVerifyState, error) {
	for i := range chainRecords {
		rec := &chainRecords[i]
		algo, normalizeErr := platformcrypto.NormalizeChainAlgorithm(rec.ChainAlgo)
		if normalizeErr != nil {
			// Out-of-schema values keep their stored name for the mixing
			// comparison and are rejected at the recompute step below, which
			// preserves the historical diagnostic order and text.
			algo = rec.ChainAlgo
		}
		if !state.Started {
			state.ExpectedAlgo = algo
		} else if algo != state.ExpectedAlgo {
			return i, state, apperrors.NewInternalf(
				"chain %s seq %d: chain_algo changed from %s to %s (mixing algorithms in one chain is forbidden)",
				chainID, rec.ChainSeq, state.ExpectedAlgo, algo)
		}
		if !state.Started {
			if rec.PrevHash != nil {
				return 0, state, apperrors.NewInternalf("chain %s seq %d: first entry has non-nil prev_hash", chainID, rec.ChainSeq)
			}
		} else if rec.PrevHash == nil || !bytesEqual(rec.PrevHash, state.ExpectedPrev) {
			return i, state, apperrors.NewInternalf("chain %s seq %d: prev_hash mismatch (chain broken)", chainID, rec.ChainSeq)
		}
		if normalizeErr != nil {
			return i, state, apperrors.WrapInternalf(normalizeErr,
				"chain %s seq %d: recompute entry_hash with %s", chainID, rec.ChainSeq, algo)
		}
		recomputed, err := computeEntryHash(
			algo,
			rec.OrganizationID, rec.ActorID, rec.Action, rec.ResourceType, rec.ResourceID,
			rec.IP, rec.UserAgent, rec.Surface, rec.RequestID, rec.TraceID,
			rec.Metadata, rec.CreatedAt, rec.PrevHash,
		)
		if err != nil {
			return i, state, apperrors.WrapInternalf(err, "chain %s seq %d: recompute entry_hash with %s", chainID, rec.ChainSeq, algo)
		}
		if rec.EntryHash == nil || !bytesEqual(rec.EntryHash, recomputed) {
			return i, state, apperrors.NewInternalf("chain %s seq %d: entry_hash mismatch with %s (record tampered)", chainID, rec.ChainSeq, algo)
		}
		state.ExpectedPrev = rec.EntryHash
		state.Started = true
	}
	return len(chainRecords), state, nil
}
