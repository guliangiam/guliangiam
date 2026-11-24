package risk

import "context"

// Repository is the persistence contract for risk signals, decisions, and
// session governance records. All methods enforce multi-tenancy via the
// organization_id filter — callers must always supply it explicitly.
type Repository interface {
	// ── Write paths (engine evaluation) ──────────────────────────────────
	AppendSignal(ctx context.Context, signal Signal) error
	CreateDecision(ctx context.Context, decision Decision) error

	// ── Read paths ───────────────────────────────────────────────────────
	ListSignals(ctx context.Context, query ListSignalsQuery) ([]*Signal, int64, error)
	ListDecisions(ctx context.Context, query ListDecisionsQuery) ([]*Decision, int64, error)
	GetDecision(ctx context.Context, id, organizationID string) (*Decision, error)
	GetDecisionForUpdate(ctx context.Context, id, organizationID string) (*Decision, error)
	UpdateDecision(ctx context.Context, cmd UpdateDecisionCommand) error
	ListGovernance(ctx context.Context, query ListGovernanceQuery) ([]*SessionGovernance, int64, error)
	GetGovernance(ctx context.Context, id, organizationID string) (*SessionGovernance, error)
	GetGovernanceForTransition(ctx context.Context, id, organizationID string) (*SessionGovernance, error)
	AppendGovernance(ctx context.Context, governance SessionGovernance) error
	ListAuditChain(ctx context.Context, query AuditIntegrityQuery) (*AuditChain, error)
}

// SignalReader is an optional read capability. PostgreSQL implements it; a
// backend that cannot answer the tenant-scoped single-row read must not fake a
// result — the app layer reports 501 instead of leaving the route undeclared.
type SignalReader interface {
	GetSignal(ctx context.Context, id, organizationID string) (*Signal, error)
}

// AuditChainSegmentReader is an optional bounded-read capability for audit
// chain integrity verification (BE-D-15). PostgreSQL implements it so a long
// chain can be streamed in fixed-size segments instead of being materialised
// in one slice. Backends that only provide ListAuditChain keep the full-load
// path; the verification result and failure semantics are identical either way.
type AuditChainSegmentReader interface {
	// ListAuditChainSegment returns up to limit entries with chain_seq >
	// afterSeq in ascending chain_seq order. Pending (not yet hashed) entries
	// are included so the app can count them instead of mistaking the chain
	// for fully verified. limit must be positive.
	ListAuditChainSegment(ctx context.Context, organizationID, chainID string, afterSeq int64, limit int) ([]AuditChainRecord, error)
	// ListAuditChainAnchors returns every anchor of the chain in ascending
	// anchor_seq order. Anchors are few, so a single bounded-by-design load is
	// acceptable even for very long chains.
	ListAuditChainAnchors(ctx context.Context, organizationID, chainID string) ([]AuditChainAnchor, error)
}
