package risk

const (
	btRiskSignals       = "`risk_signals`"
	btRiskDecisions     = "`risk_decisions`"
	btSessionGovernance = "`session_governance`"
	btRecords           = "`records`"
)

const queryAppendSignal = `
INSERT INTO ` + btRiskSignals + `
	(id, organization_id, user_id, signal_type, signal_value, ip, created_at)
VALUES (?, ?, ?, ?, CAST(? AS JSON), ?, ?)
`

const queryCreateDecision = `
INSERT INTO ` + btRiskDecisions + `
	(id, organization_id, user_id, score, decision, factors, created_at)
VALUES (?, ?, ?, ?, ?, CAST(? AS JSON), ?)
`

const queryListSignalsBase = "FROM " + btRiskSignals + " WHERE organization_id = ?"

const queryListSignalsSelect = `
SELECT id, organization_id, user_id, signal_type, signal_value, ip, created_at
`

const queryListSignalsOrderPage = `
LIMIT ? OFFSET ?
`

const queryListDecisionsBase = "FROM " + btRiskDecisions + " WHERE organization_id = ?"

const queryListDecisionsSelect = `
SELECT id, organization_id, user_id, score, decision, factors, created_at
`

const queryGetDecision = `
SELECT id, organization_id, user_id, score, decision, factors, created_at
FROM ` + btRiskDecisions + `
WHERE id = ? AND organization_id = ?
`

const queryGetDecisionForUpdate = queryGetDecision + ` FOR UPDATE`

const queryUpdateDecision = `
UPDATE ` + btRiskDecisions + `
SET decision = ?, factors = CAST(? AS JSON)
WHERE id = ? AND organization_id = ?
  AND decision = ? AND factors = CAST(? AS JSON)
`

const queryListGovernanceBase = "FROM " + btSessionGovernance + " g WHERE g.organization_id = ?"

const queryGovernanceRevision = `
CASE WHEN g.session_id IS NULL THEN 1 ELSE (
	SELECT COUNT(*) FROM ` + btSessionGovernance + ` h
	WHERE h.organization_id = g.organization_id AND h.session_id = g.session_id
	  AND (h.created_at < g.created_at OR (h.created_at = g.created_at AND h.id <= g.id))
) END
`

const queryListGovernanceSelect = `
SELECT g.id, g.organization_id, g.session_id, g.risk_score, g.action, g.reason, g.created_at, ` + queryGovernanceRevision

const queryGetGovernance = queryListGovernanceSelect + `
FROM ` + btSessionGovernance + ` g
WHERE g.id = ? AND g.organization_id = ?
`

const queryLockGovernance = queryGetGovernance + ` FOR UPDATE`

const queryGetLatestGovernance = queryListGovernanceSelect + `
FROM ` + btSessionGovernance + ` g
WHERE g.organization_id = ? AND g.session_id = ?
ORDER BY g.created_at DESC, g.id DESC
LIMIT 1 FOR UPDATE
`

const queryAppendGovernance = `
INSERT INTO ` + btSessionGovernance + `
	(id, organization_id, session_id, risk_score, action, reason, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`

const queryListAuditChain = `
SELECT organization_id, COALESCE(actor_id, ''), action,
       COALESCE(resource_type, ''), COALESCE(resource_id, ''),
       COALESCE(ip, ''), COALESCE(user_agent, ''),
       COALESCE(surface, ''), COALESCE(request_id, ''), COALESCE(trace_id, ''),
       COALESCE(metadata, '{}'), created_at,
       chain_seq, prev_hash, entry_hash, chain_algo
FROM ` + btRecords + `
WHERE organization_id = ? AND chain_id = ?
ORDER BY chain_seq ASC
`
