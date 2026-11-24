package risk

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	platformaudit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── BE-D-15: segmented audit chain verification regressions ──────────

type segmentCall struct {
	afterSeq int64
	limit    int
}

// recordingSegmentRepository implements the bounded-read capability and
// records every page request so tests can assert the segment query sequence.
type recordingSegmentRepository struct {
	domain.Repository
	entries     []domain.AuditChainRecord
	anchors     []domain.AuditChainAnchor
	calls       []segmentCall
	anchorCalls int
}

func (r *recordingSegmentRepository) ListAuditChainSegment(_ context.Context, _, _ string, afterSeq int64, limit int) ([]domain.AuditChainRecord, error) {
	r.calls = append(r.calls, segmentCall{afterSeq: afterSeq, limit: limit})
	entries := make([]domain.AuditChainRecord, 0, limit)
	for _, entry := range r.entries {
		if entry.ChainSeq <= afterSeq {
			continue
		}
		entries = append(entries, entry)
		if len(entries) >= limit {
			break
		}
	}
	return entries, nil
}

func (r *recordingSegmentRepository) ListAuditChainAnchors(_ context.Context, _, _ string) ([]domain.AuditChainAnchor, error) {
	r.anchorCalls++
	return r.anchors, nil
}

// legacyAuditChainRepository only implements the full-load port, exercising
// the fallback path kept for backends without AuditChainSegmentReader.
type legacyAuditChainRepository struct {
	domain.Repository
	chain *domain.AuditChain
}

func (r *legacyAuditChainRepository) ListAuditChain(_ context.Context, _ domain.AuditIntegrityQuery) (*domain.AuditChain, error) {
	if r.chain == nil {
		return &domain.AuditChain{}, nil
	}
	return r.chain, nil
}

type stubRiskUnitOfWork struct {
	*fakeUnitOfWork
	repo domain.Repository
}

func (u *stubRiskUnitOfWork) Risk() domain.Repository { return u.repo }

func newSegmentService(repo domain.Repository) Service {
	fixture := newServiceFixture()
	return Service{Clock: fakeClock{}, UUID: fakeUUID{}, UOW: &stubRiskUnitOfWork{fakeUnitOfWork: fixture.uow, repo: repo}}
}

// buildChainedEntries produces a fully hashed, gap-free sha256 chain of the
// requested length using the same canonical hash the writer/verifier use.
func buildChainedEntries(t *testing.T, count int) []domain.AuditChainRecord {
	t.Helper()
	entries := make([]domain.AuditChainRecord, 0, count)
	var prev []byte
	for seq := 1; seq <= count; seq++ {
		entry := domain.AuditChainRecord{
			OrganizationID: testOrgID, ActorID: testActorID, Action: "risk.decision.reviewed",
			ResourceType: "decision", ResourceID: testDecisionID, Surface: "management",
			Metadata: map[string]any{}, CreatedAt: testNow, ChainSeq: int64(seq),
			PrevHash: prev, ChainAlgo: "sha256",
		}
		hash, err := platformaudit.RecomputeEntryHash("sha256", platformaudit.ChainRecord{
			OrganizationID: entry.OrganizationID, ActorID: entry.ActorID, Action: entry.Action,
			ResourceType: entry.ResourceType, ResourceID: entry.ResourceID, Surface: entry.Surface,
			Metadata: entry.Metadata, CreatedAt: entry.CreatedAt, ChainSeq: entry.ChainSeq,
			PrevHash: entry.PrevHash, ChainAlgo: entry.ChainAlgo,
		})
		require.NoError(t, err)
		entry.EntryHash = hash
		prev = hash
		entries = append(entries, entry)
	}
	return entries
}

func verifyWithRepo(t *testing.T, repo domain.Repository) *AuditIntegrityDTO {
	t.Helper()
	service := newSegmentService(repo)
	result, err := service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID})
	require.NoError(t, err)
	return result
}

func TestVerifyAuditIntegrity_SegmentedLongChainValid(t *testing.T) {
	repo := &recordingSegmentRepository{entries: buildChainedEntries(t, 1001)}
	result := verifyWithRepo(t, repo)
	assert.True(t, result.Valid)
	assert.Equal(t, "valid", result.Status)
	assert.Equal(t, 1001, result.TotalCount)
	assert.Equal(t, 1001, result.VerifiedCount)
	assert.Zero(t, result.PendingCount)
	assert.Nil(t, result.Failure)
	assert.Equal(t, 1, repo.anchorCalls)
	// Pages advance by the last seen chain_seq with the fixed segment size.
	assert.Equal(t, []segmentCall{{0, 500}, {500, 500}, {1000, 500}}, repo.calls)
}

func TestVerifyAuditIntegrity_SegmentedSequenceGapLocated(t *testing.T) {
	entries := buildChainedEntries(t, 1001)
	// Drop seq 600: the next hashed record (601) lands in the second segment.
	entries = append(entries[:599], entries[600:]...)
	repo := &recordingSegmentRepository{entries: entries}
	result := verifyWithRepo(t, repo)
	assert.Equal(t, "invalid", result.Status)
	require.NotNil(t, result.Failure)
	assert.Equal(t, int64(601), result.Failure.ChainSeq)
	assert.Contains(t, result.Failure.Reason, "sequence mismatch: expected 600")
	assert.Equal(t, 599, result.VerifiedCount)
	// Whole-chain totals survive the failure (remaining pages are drained).
	assert.Equal(t, 1000, result.TotalCount)
	assert.Zero(t, result.PendingCount)
	// Segment 2 spans seq 501..1001 minus the dropped 600, so the drain page
	// resumes after 1001 and comes back empty.
	assert.Equal(t, []segmentCall{{0, 500}, {500, 500}, {1001, 500}}, repo.calls)
}

func TestVerifyAuditIntegrity_SegmentedAlgorithmChangeLocated(t *testing.T) {
	entries := buildChainedEntries(t, 1001)
	entries[799].ChainAlgo = "sm3" // seq 800, second segment
	repo := &recordingSegmentRepository{entries: entries}
	result := verifyWithRepo(t, repo)
	assert.Equal(t, "invalid", result.Status)
	require.NotNil(t, result.Failure)
	assert.Equal(t, int64(800), result.Failure.ChainSeq)
	assert.Contains(t, result.Failure.Reason, "algorithm changed from sha256 to sm3 at chain_seq 800")
	assert.Equal(t, 799, result.VerifiedCount)
	assert.Equal(t, 1001, result.TotalCount)
}

func TestVerifyAuditIntegrity_SegmentedTamperLocated(t *testing.T) {
	entries := buildChainedEntries(t, 1001)
	entries[699].EntryHash[0] ^= 0xFF // seq 700, second segment
	repo := &recordingSegmentRepository{entries: entries}
	result := verifyWithRepo(t, repo)
	assert.Equal(t, "invalid", result.Status)
	require.NotNil(t, result.Failure)
	assert.Equal(t, int64(700), result.Failure.ChainSeq)
	assert.Contains(t, result.Failure.Reason, "entry_hash mismatch")
	assert.Equal(t, 699, result.VerifiedCount)
	assert.Equal(t, 1001, result.TotalCount)
}

func TestVerifyAuditIntegrity_SegmentedEarlyFailureStillCountsWholeChain(t *testing.T) {
	entries := buildChainedEntries(t, 1001)
	entries[99].EntryHash[0] ^= 0xFF // seq 100, first segment
	repo := &recordingSegmentRepository{entries: entries}
	result := verifyWithRepo(t, repo)
	assert.Equal(t, "invalid", result.Status)
	require.NotNil(t, result.Failure)
	assert.Equal(t, int64(100), result.Failure.ChainSeq)
	assert.Equal(t, 99, result.VerifiedCount)
	assert.Equal(t, 1001, result.TotalCount)
	assert.Equal(t, []segmentCall{{0, 500}, {500, 500}, {1000, 500}}, repo.calls)
}

func TestVerifyAuditIntegrity_SegmentedAnchorsMatchAcrossSegments(t *testing.T) {
	entries := buildChainedEntries(t, 1001)
	repo := &recordingSegmentRepository{
		entries: entries,
		anchors: []domain.AuditChainAnchor{
			{AnchorSeq: 250, EntryHash: entries[249].EntryHash},
			{AnchorSeq: 750, EntryHash: entries[749].EntryHash},
			{AnchorSeq: 1001, EntryHash: entries[1000].EntryHash},
		},
	}
	result := verifyWithRepo(t, repo)
	assert.True(t, result.Valid)
	assert.Equal(t, "valid", result.Status)
	assert.Equal(t, 1001, result.VerifiedCount)
	assert.Nil(t, result.Failure)
}

func TestVerifyAuditIntegrity_SegmentedAnchorMismatchInLaterSegment(t *testing.T) {
	entries := buildChainedEntries(t, 1001)
	repo := &recordingSegmentRepository{
		entries: entries,
		anchors: []domain.AuditChainAnchor{
			{AnchorSeq: 250, EntryHash: entries[249].EntryHash},
			{AnchorSeq: 750, EntryHash: []byte("tampered")},
		},
	}
	result := verifyWithRepo(t, repo)
	assert.Equal(t, "invalid", result.Status)
	require.NotNil(t, result.Failure)
	assert.Equal(t, int64(750), result.Failure.ChainSeq)
	assert.Contains(t, result.Failure.Reason, "anchor entry_hash mismatch")
	// The chain itself fully verified before the anchor check ran.
	assert.Equal(t, 1001, result.VerifiedCount)
}

func TestVerifyAuditIntegrity_SegmentedAnchorTargetsPendingTailEntry(t *testing.T) {
	entries := buildChainedEntries(t, 1000)
	entries = append(entries, domain.AuditChainRecord{
		OrganizationID: testOrgID, ChainSeq: 1001, ChainAlgo: "sha256", // pending
	})
	repo := &recordingSegmentRepository{
		entries: entries,
		anchors: []domain.AuditChainAnchor{{AnchorSeq: 1001, EntryHash: []byte{9}}},
	}
	result := verifyWithRepo(t, repo)
	assert.Equal(t, "invalid", result.Status)
	require.NotNil(t, result.Failure)
	assert.Equal(t, int64(1001), result.Failure.ChainSeq)
	assert.Contains(t, result.Failure.Reason, "missing or pending")
	assert.Equal(t, 1000, result.VerifiedCount)
	assert.Equal(t, 1, result.PendingCount)
	assert.Equal(t, 1001, result.TotalCount)
}

func TestVerifyAuditIntegrity_SegmentedAnchorBeyondChainEnd(t *testing.T) {
	repo := &recordingSegmentRepository{
		entries: buildChainedEntries(t, 10),
		anchors: []domain.AuditChainAnchor{{AnchorSeq: 11, EntryHash: []byte{1}}},
	}
	result := verifyWithRepo(t, repo)
	assert.Equal(t, "invalid", result.Status)
	require.NotNil(t, result.Failure)
	assert.Equal(t, int64(11), result.Failure.ChainSeq)
	assert.Contains(t, result.Failure.Reason, "missing or pending")
}

func TestVerifyAuditIntegrity_SegmentedPendingCountAcrossSegments(t *testing.T) {
	entries := buildChainedEntries(t, 1000)
	entries = append(entries,
		domain.AuditChainRecord{OrganizationID: testOrgID, ChainSeq: 1001, ChainAlgo: "sha256"},
		domain.AuditChainRecord{OrganizationID: testOrgID, ChainSeq: 1002, ChainAlgo: "sha256"},
	)
	repo := &recordingSegmentRepository{entries: entries}
	result := verifyWithRepo(t, repo)
	assert.False(t, result.Valid)
	assert.Equal(t, "pending", result.Status)
	assert.Nil(t, result.Failure)
	assert.Equal(t, 2, result.PendingCount)
	assert.Equal(t, 1000, result.VerifiedCount)
	assert.Equal(t, 1002, result.TotalCount)
}

func TestVerifyAuditIntegrity_SegmentedMidChainPendingKeepsStructuralSemantics(t *testing.T) {
	entries := buildChainedEntries(t, 1001)
	entries[600].EntryHash = nil // seq 601 becomes pending mid-chain
	repo := &recordingSegmentRepository{entries: entries}
	result := verifyWithRepo(t, repo)
	// A pending hole still breaks the one-based hashed-record sequence, just
	// as in the full-load implementation: the next hashed record is located.
	assert.Equal(t, "invalid", result.Status)
	require.NotNil(t, result.Failure)
	assert.Equal(t, int64(602), result.Failure.ChainSeq)
	assert.Contains(t, result.Failure.Reason, "sequence mismatch: expected 601")
	assert.Equal(t, 600, result.VerifiedCount)
	assert.Equal(t, 1, result.PendingCount)
	assert.Equal(t, 1001, result.TotalCount)
}

func TestVerifyAuditIntegrity_SegmentedChainFailureOutranksAnchorFailure(t *testing.T) {
	entries := buildChainedEntries(t, 10)
	entries[4].EntryHash[0] ^= 0xFF // seq 5 tampered
	repo := &recordingSegmentRepository{
		entries: entries,
		anchors: []domain.AuditChainAnchor{{AnchorSeq: 3, EntryHash: []byte("bad")}},
	}
	result := verifyWithRepo(t, repo)
	assert.Equal(t, "invalid", result.Status)
	require.NotNil(t, result.Failure)
	assert.Equal(t, int64(5), result.Failure.ChainSeq)
	assert.Contains(t, result.Failure.Reason, "entry_hash mismatch")
}

func TestVerifyAuditIntegrity_LegacyFullLoadFallback(t *testing.T) {
	entries := buildChainedEntries(t, 3)
	t.Run("valid chain with anchor", func(t *testing.T) {
		repo := &legacyAuditChainRepository{chain: &domain.AuditChain{
			Entries: entries,
			Anchors: []domain.AuditChainAnchor{{AnchorSeq: 3, EntryHash: entries[2].EntryHash}},
		}}
		result := verifyWithRepo(t, repo)
		assert.True(t, result.Valid)
		assert.Equal(t, "valid", result.Status)
		assert.Equal(t, 3, result.TotalCount)
		assert.Equal(t, 3, result.VerifiedCount)
		assert.Nil(t, result.Failure)
	})
	t.Run("tamper is located", func(t *testing.T) {
		tampered := append([]domain.AuditChainRecord(nil), entries...)
		tampered[1].EntryHash = []byte("tampered")
		repo := &legacyAuditChainRepository{chain: &domain.AuditChain{Entries: tampered}}
		result := verifyWithRepo(t, repo)
		assert.Equal(t, "invalid", result.Status)
		require.NotNil(t, result.Failure)
		assert.Equal(t, int64(2), result.Failure.ChainSeq)
		assert.Equal(t, 1, result.VerifiedCount)
	})
	t.Run("pending tail with anchor", func(t *testing.T) {
		withPending := append(append([]domain.AuditChainRecord(nil), entries...),
			domain.AuditChainRecord{OrganizationID: testOrgID, ChainSeq: 4, ChainAlgo: "sha256"})
		repo := &legacyAuditChainRepository{chain: &domain.AuditChain{
			Entries: withPending,
			Anchors: []domain.AuditChainAnchor{{AnchorSeq: 4, EntryHash: []byte{1}}},
		}}
		result := verifyWithRepo(t, repo)
		assert.Equal(t, "invalid", result.Status)
		require.NotNil(t, result.Failure)
		assert.Equal(t, int64(4), result.Failure.ChainSeq)
		assert.Contains(t, result.Failure.Reason, "missing or pending")
		assert.Equal(t, 1, result.PendingCount)
	})
}
