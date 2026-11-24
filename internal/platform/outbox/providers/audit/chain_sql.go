package audit

// ChainSQL is the SQL text provider injected by the infra layer.
//
// It lives in the platform audit package so AuditChainWriter no longer imports
// domain/records (the previous location of the SQL constants). The concrete
// statements are declared in infra/postgres/records/chain_sql.go and injected
// by the composition root; platform never imports infra.
type ChainSQL struct {
	AdvisoryLock  string
	LoadLastEntry string
	LoadAnchorSeq string
	// InsertAnchorAndAdvance inserts the anchor and advances the covered
	// records to chain_status='anchored' in one data-modifying CTE.
	InsertAnchorAndAdvance string
	// AdvanceChainedStatus backfills chain_status='anchored' for records covered
	// by an already existing anchor (no new anchor is inserted).
	AdvanceChainedStatus string
	ListEntries          string
	// ListEntriesBatch loads a bounded page of full chain rows after a
	// chain_seq cursor. Full fields (metadata included) are required so the
	// background verifier can recompute entry_hash; the link-only ListEntries
	// statement cannot detect field tampering.
	ListEntriesBatch string
	ListActiveChains string
	// CountPending counts records still in chain_status='pending'. The status
	// filter must be explicit: a missing entry_hash is not the same state
	// (broken rows also lack one).
	CountPending string
}
