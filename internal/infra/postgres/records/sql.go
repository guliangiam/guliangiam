package records

import "strings"

// recordsFilter is the shared WHERE predicate of the four list/count queries.
//
// actor_id is a UUID column, so it cannot be compared to the incoming text
// parameter directly: `actor_id = $2` does not even parse ("operator does not
// exist: uuid = text"), and `$2::uuid` would turn a malformed filter value into
// a 500. Comparing through `actor_id::text` keeps the statement type-safe for
// arbitrary input; the keyword branch uses the same expression for the same
// reason. organization_id goes through guliangiam.resolve_org_id so a slug and
// a UUID both work.
const recordsFilter = `
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR actor_id::text = $2)
  AND ($3 = '' OR action = $3)
  AND ($4 = '' OR resource_type = $4)`

// recordsKeyword is the ILIKE branch shared by the list and search queries.
// The `{K}` placeholder carries the keyword parameter ordinal ($5 for the list
// queries, $8 for the search queries), so the four statements cannot drift
// apart: earlier they were copy-pasted and one copy still compared a UUID column
// against text.
const recordsKeyword = `
  AND ({K} = '' OR CAST(id AS text) ILIKE '%' || {K} || '%'
       OR actor_id::text ILIKE '%' || {K} || '%'
       OR action ILIKE '%' || {K} || '%'
       OR resource_type ILIKE '%' || {K} || '%'
       OR resource_id ILIKE '%' || {K} || '%')`

// recordsTimeRange is the optional [start, end] window of the search queries.
const recordsTimeRange = `
  AND ($6::timestamptz IS NULL OR created_at >= $6::timestamptz)
  AND ($7::timestamptz IS NULL OR created_at <= $7::timestamptz)`

// keywordFor binds the keyword predicate to a parameter ordinal.
func keywordFor(ordinal string) string {
	return strings.ReplaceAll(recordsKeyword, "{K}", ordinal)
}

// queryCreateRecord 与 Compute 算出的链字段同批写入 chain_status='chained'：
// 记录一旦落库就已参与链，不再停留 pending。列默认值保持 'pending'，供没有
// entry_hash 的历史行（或未来的异步补链路径）如实表达"尚未入链"。
const queryCreateRecord = `
INSERT INTO guliangiam.records (id, organization_id, actor_id, action, resource_type, resource_id, ip, user_agent, surface, request_id, trace_id, metadata, created_at, chain_id, chain_seq, prev_hash, entry_hash, chain_algo, chain_status)
VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, 'chained')`

// The ORDER BY is an `{ORDER_BY}` slot filled by Repository.List/Search from the
// sort whitelist. It used to be the literal `ORDER BY created_at DESC`, which made
// the `sortField`/`sortOrder` parameters the handlers read and forwarded all the
// way down **do nothing at all** — the endpoint advertised sorting it could not
// perform (开发规范手册 4.8.2「SQL 注入红线」；开放项注册表 OPEN-API-05). The substituted
// value is never
// client-supplied: it is built from the whitelist inside platform/sorting.
//
// It is a named token rather than a `%s` verb because these statements contain
// literal `%` characters — the ILIKE wildcards in recordsKeyword and
// recordsFilter. `fmt.Sprintf` would treat each of those as a verb and corrupt
// the pattern, which is also why the users query escapes them as `%%`.
var (
	queryListRecords = `
SELECT id, organization_id, actor_id, action, resource_type, resource_id, ip::text, user_agent, surface, request_id, trace_id, metadata, created_at
FROM guliangiam.records` + recordsFilter + `
  AND ($5 = '' OR request_id = $5)` + keywordFor("$6") + `
{ORDER_BY}
LIMIT $7 OFFSET $8`

	queryCountRecords = `
SELECT COUNT(*)
FROM guliangiam.records` + recordsFilter + `
  AND ($5 = '' OR request_id = $5)` + keywordFor("$6")

	querySearchRecords = `
SELECT id, organization_id, actor_id, action, resource_type, resource_id, ip::text, user_agent, surface, request_id, trace_id, metadata, created_at
FROM guliangiam.records` + recordsFilter + `
  AND ($5 = '' OR resource_id = $5)
  AND ($8 = '' OR request_id = $8)` + recordsTimeRange + keywordFor("$9") + `
{ORDER_BY}
LIMIT $10 OFFSET $11`

	queryCountSearchRecords = `
SELECT COUNT(*)
FROM guliangiam.records` + recordsFilter + `
  AND ($5 = '' OR resource_id = $5)
  AND ($8 = '' OR request_id = $8)` + recordsTimeRange + keywordFor("$9")
)

// ── 哈希链维护（AuditChainWriter 内部调用） ──────────────────────────

// queryLoadLastChainEntry 加载指定 chain 最新条目的 (chain_seq, entry_hash)。
// 并发安全由 audit.AuditChainWriter.loadLastEntry 中的 advisory lock 保证。
const queryLoadLastChainEntry = `
	SELECT chain_seq, entry_hash
	FROM guliangiam.records
	WHERE chain_id = $1
	ORDER BY chain_seq DESC
	LIMIT 1`

// queryAdvisoryLock 串行化同一 chain_id 的并发 Compute() —— 避免
// "SELECT MAX(chain_seq) + 1" 竞态导致 uq_records_chain_seq 唯一约束冲突。
// pg_advisory_xact_lock 事务内有效，事务结束自动释放。
const queryAdvisoryLock = `SELECT pg_advisory_xact_lock(hashtext($1))`

// queryLoadAnchorSeq 返回指定 chain 最大 anchor_seq（空链返回 0）。
const queryLoadAnchorSeq = `
	SELECT COALESCE(MAX(anchor_seq), 0)
	FROM guliangiam.audit_chain_anchors
	WHERE organization_id = $1 AND chain_id = $2`

// queryInsertAnchor 写入新锚点。
const queryInsertAnchor = `
	INSERT INTO guliangiam.audit_chain_anchors
		(id, organization_id, chain_id, anchor_seq, entry_hash, proof)
	VALUES ($1, $2::uuid, $3, $4, $5, $6)`

// queryListChainEntries 按 chain_seq ASC 拉全部条目（用于 VerifyChain）。
const queryListChainEntries = `
	SELECT chain_seq, prev_hash, entry_hash, chain_algo
	FROM guliangiam.records
	WHERE chain_id = $1 AND entry_hash IS NOT NULL
	ORDER BY chain_seq ASC`
