package risk

const queryAppendSignal = `
INSERT INTO guliangiam.risk_signals
	(id, organization_id, user_id, signal_type, signal_value, ip, created_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
`

const queryCreateDecision = `
INSERT INTO guliangiam.risk_decisions
	(id, organization_id, user_id, score, decision, factors, created_at)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
`

const queryListSignalsBase = `FROM guliangiam.risk_signals WHERE organization_id = $1`

const queryListSignalsSelect = `
SELECT id, organization_id, user_id, signal_type, signal_value, ip::text, created_at
`

const queryListSignalsOrderPage = `
LIMIT $%d OFFSET $%d
`

const queryListDecisionsBase = `FROM guliangiam.risk_decisions WHERE organization_id = $1`

const queryListDecisionsSelect = `
SELECT id, organization_id, user_id, score, decision, factors, created_at
`

const queryGetDecision = `
SELECT id, organization_id, user_id, score, decision, factors, created_at
FROM guliangiam.risk_decisions
WHERE id = $1 AND organization_id = $2
`

const queryGetDecisionForUpdate = queryGetDecision + ` FOR UPDATE`

const queryUpdateDecision = `
UPDATE guliangiam.risk_decisions
SET decision = $1, factors = $2::jsonb
WHERE id = $3 AND organization_id = $4
  AND decision = $5 AND factors = $6::jsonb
`

const queryListGovernanceBase = `FROM guliangiam.session_governance g WHERE g.organization_id = $1`

const queryGovernanceRevision = `
CASE WHEN g.session_id IS NULL THEN 1 ELSE (
	SELECT COUNT(*) FROM guliangiam.session_governance h
	WHERE h.organization_id = g.organization_id AND h.session_id = g.session_id
	  AND (h.created_at < g.created_at OR (h.created_at = g.created_at AND h.id <= g.id))
) END
`

const queryListGovernanceSelect = `
SELECT g.id, g.organization_id, g.session_id, g.risk_score, g.action, g.reason, g.created_at, ` + queryGovernanceRevision

// queryGovernanceRevisionWindow is the single-pass equivalent of
// queryGovernanceRevision for list pages (BE-D-15): (created_at, id) is unique
// within a session (id is the PK) and the tenant is already filtered, so the
// running COUNT over the session partition yields the same one-based revision
// without one correlated COUNT per returned row. It must be evaluated over the
// full tenant-scoped history BEFORE optional list filters (action) drop sibling
// rows — queryListGovernanceWindowPage therefore wraps it in a derived table
// and the filter is applied outside. Single-row and FOR UPDATE queries keep
// queryGovernanceRevision: a window function combined with FOR UPDATE + LIMIT
// would change which rows get locked.
const queryGovernanceRevisionWindow = `
CASE WHEN g.session_id IS NULL THEN 1 ELSE COUNT(*) OVER (
	PARTITION BY g.session_id ORDER BY g.created_at, g.id
	ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
) END`

const queryListGovernanceWindowSelect = `
SELECT g.id, g.organization_id, g.session_id, g.risk_score, g.action, g.reason, g.created_at, ` + queryGovernanceRevisionWindow + ` AS revision
`

const queryListGovernanceWindowPage = `
SELECT g.id, g.organization_id, g.session_id, g.risk_score, g.action, g.reason, g.created_at, g.revision
FROM (` + queryListGovernanceWindowSelect + queryListGovernanceBase + `) g`

const queryGetGovernance = queryListGovernanceSelect + `
FROM guliangiam.session_governance g
WHERE g.id = $1 AND g.organization_id = $2
`

const queryLockGovernance = queryGetGovernance + ` FOR UPDATE`

const queryGetLatestGovernance = queryListGovernanceSelect + `
FROM guliangiam.session_governance g
WHERE g.organization_id = $1 AND g.session_id = $2
ORDER BY g.created_at DESC, g.id DESC
LIMIT 1 FOR UPDATE
`

const queryAppendGovernance = `
INSERT INTO guliangiam.session_governance
	(id, organization_id, session_id, risk_score, action, reason, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

// queryListAuditChainSelect is intentionally read-only and scoped by both
// tenant and chain. NULL entry_hash rows are returned so the app can report
// pending work instead of mistaking it for a healthy, fully verified chain.
const queryListAuditChainSelect = `
SELECT organization_id::text, COALESCE(actor_id::text, ''), action,
       COALESCE(resource_type, ''), COALESCE(resource_id, ''),
       COALESCE(HOST(ip), ''), COALESCE(user_agent, ''),
       COALESCE(surface, ''), COALESCE(request_id, ''), COALESCE(trace_id, ''),
       COALESCE(metadata, '{}'::jsonb), created_at,
       chain_seq, prev_hash, entry_hash, chain_algo
FROM guliangiam.records
WHERE organization_id = $1 AND chain_id = $2`

const queryListAuditChain = queryListAuditChainSelect + `
ORDER BY chain_seq ASC
`

// queryListAuditChainSegment is the bounded-read variant (BE-D-15): the app
// verifier walks a long chain page by page with chain_seq > afterSeq instead
// of loading every record into memory at once.
const queryListAuditChainSegment = queryListAuditChainSelect + ` AND chain_seq > $3
ORDER BY chain_seq ASC
LIMIT $4
`

const queryListAuditChainAnchors = `
SELECT anchor_seq, entry_hash
FROM guliangiam.audit_chain_anchors
WHERE organization_id = $1 AND chain_id = $2
ORDER BY anchor_seq ASC
`
