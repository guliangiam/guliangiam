package records

import "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"

// Audit chain SQL lives here: SQL text is only allowed under internal/infra/**.
// The writer receives this value through the composition root so platform and
// domain never own or import SQL statements.
const (
	chainSQLAdvisoryLock = `SELECT pg_advisory_xact_lock(hashtext($1))`

	chainSQLLoadLastEntry = `
		SELECT chain_seq, entry_hash, chain_algo
		FROM guliangiam.records
		WHERE chain_id = $1
		ORDER BY chain_seq DESC
		LIMIT 1`

	// anchor_seq 记录锚点闭合的链尾 chain_seq（不是锚点序号）：既让
	// uq_audit_chain_anchors_org_chain_seq 拒绝对同一链尾重复锚定，也让下面的
	// UPDATE 条件恰好覆盖本锚点所保护的记录区间。
	chainSQLLoadAnchorSeq = `
		SELECT COALESCE(MAX(anchor_seq), 0)
		FROM guliangiam.audit_chain_anchors
		WHERE organization_id = $1 AND chain_id = $2`

	// 单条 data-modifying CTE：锚点插入与 records 状态推进在同一个语句内完成，
	// 不存在"锚点已落库但记录仍显示 chained"的中间态。PostgreSQL 对 WITH 中的
	// data-modifying 子句保证执行（即使主语句不引用 ins）；链尾 chain_seq 写入
	// $4，UPDATE 的 chain_seq <= $4 即本锚点覆盖的记录范围。只推进 chained 行：
	// pending（无 entry_hash）与 broken 不得被锚点掩盖。
	chainSQLInsertAnchorAndAdvance = `
		WITH ins AS (
			INSERT INTO guliangiam.audit_chain_anchors
				(id, organization_id, chain_id, anchor_seq, entry_hash, proof)
			VALUES ($1, $2, $3, $4, $5, $6)
		)
		UPDATE guliangiam.records
		SET chain_status = 'anchored'
		WHERE chain_id = $3 AND chain_seq <= $4 AND chain_status = 'chained'`

	// 链尾已被既有锚点覆盖（Worker 周期运行时没有新记录）时的幂等回填：
	// 不新增锚点，只把该锚点覆盖范围内仍是 chained 的行推进为 anchored。
	chainSQLAdvanceChainedStatus = `
		UPDATE guliangiam.records
		SET chain_status = 'anchored'
		WHERE chain_id = $1 AND chain_seq <= $2 AND chain_status = 'chained'`

	chainSQLListEntries = `
		SELECT chain_seq, prev_hash, entry_hash, chain_algo
		FROM guliangiam.records
		WHERE chain_id = $1 AND entry_hash IS NOT NULL
		ORDER BY chain_seq ASC`

	// 全量校验的有界分页读：keyset 游标 chain_seq > $2，按升序取 $3 条。
	// 列集合必须与 Compute 的规范载荷逐项对应（含 metadata/created_at），
	// 否则重算出的 entry_hash 会与写入口径漂移。
	chainSQLListEntriesBatch = `
		SELECT organization_id::text, COALESCE(actor_id::text, ''), action,
		       COALESCE(resource_type, ''), COALESCE(resource_id, ''),
		       COALESCE(HOST(ip), ''), COALESCE(user_agent, ''),
		       COALESCE(surface, ''), COALESCE(request_id, ''), COALESCE(trace_id, ''),
		       COALESCE(metadata, '{}'::jsonb), created_at,
		       chain_seq, prev_hash, entry_hash, chain_algo
		FROM guliangiam.records
		WHERE chain_id = $1 AND entry_hash IS NOT NULL AND chain_seq > $2
		ORDER BY chain_seq ASC
		LIMIT $3`

	chainSQLListActiveChains = `
		SELECT DISTINCT chain_id
		FROM guliangiam.records
		WHERE entry_hash IS NOT NULL
		ORDER BY chain_id`

	// 只按声明的链状态计数：broken 行同样没有可用的 entry_hash，用
	// entry_hash IS NULL 近似会把两类状态混为一谈。
	chainSQLCountPending = `
		SELECT COUNT(*)
		FROM guliangiam.records
		WHERE chain_status = 'pending'`
)

// DefaultChainSQL returns the PostgreSQL audit-chain statements.
func DefaultChainSQL() audit.ChainSQL {
	return audit.ChainSQL{
		AdvisoryLock:           chainSQLAdvisoryLock,
		LoadLastEntry:          chainSQLLoadLastEntry,
		LoadAnchorSeq:          chainSQLLoadAnchorSeq,
		InsertAnchorAndAdvance: chainSQLInsertAnchorAndAdvance,
		AdvanceChainedStatus:   chainSQLAdvanceChainedStatus,
		ListEntries:            chainSQLListEntries,
		ListEntriesBatch:       chainSQLListEntriesBatch,
		ListActiveChains:       chainSQLListActiveChains,
		CountPending:           chainSQLCountPending,
	}
}
