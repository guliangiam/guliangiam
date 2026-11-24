package rebac

// ─── relation_namespaces ──────────────────────────────────────────────────────
// 所有 SELECT 返回 revision_counter（Zookie 写后读一致性计数器）

const queryCreateNamespace = `
INSERT INTO guliangiam.relation_namespaces
  (id, organization_id, name, display_name, relation_defs, version, created_at, updated_at, revision_counter)
VALUES ($1, $2, $3, $4, $5::jsonb, $6, now(), now(), 0)
RETURNING id, organization_id, name, display_name, relation_defs, version, created_at, updated_at, revision_counter
`

const queryListNamespaces = `
SELECT id, organization_id, name, display_name, relation_defs, version, created_at, updated_at, revision_counter
FROM guliangiam.relation_namespaces
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

const queryListNamespacesCount = `
SELECT count(*) FROM guliangiam.relation_namespaces WHERE organization_id = $1
`

const queryGetNamespace = `
SELECT id, organization_id, name, display_name, relation_defs, version, created_at, updated_at, revision_counter
FROM guliangiam.relation_namespaces
WHERE id = $1 AND organization_id = $2
`

// queryGetNamespaceByName looks up a namespace by (org, name).
// Used by APIAuthorizationMiddleware's namespace resolver.
const queryGetNamespaceByName = `
SELECT id, organization_id, name, display_name, relation_defs, version, created_at, updated_at, revision_counter
FROM guliangiam.relation_namespaces
WHERE organization_id = $1 AND name = $2
`

const queryDeleteNamespace = `
DELETE FROM guliangiam.relation_namespaces
WHERE id = $1 AND organization_id = $2
`

const queryGetLatestZookie = `
SELECT revision_counter FROM guliangiam.relation_namespaces
WHERE id = $1 AND organization_id = $2
`

// ─── relation_tuples ─────────────────────────────────────────────────────────

// queryCreateTuple 是一条原子 CTE：
//  1. bump namespace 的 revision_counter（单语句原子，无需显式事务）
//  2. INSERT tuple，zookie = bump 后的值
//  3. ON CONFLICT DO NOTHING 保证幂等
//
// 注意：即使 ON CONFLICT DO NOTHING，revision_counter 仍然 bump ——
// 这是 OpenFGA 兼容行为（revision 不需要连续，只需单调递增）。
const queryCreateTuple = `
WITH bump AS (
  UPDATE guliangiam.relation_namespaces
  SET revision_counter = revision_counter + 1
  WHERE id = $1 AND organization_id = $2
  RETURNING revision_counter
)
INSERT INTO guliangiam.relation_tuples
  (id, organization_id, namespace_id, subject, subject_relation, relation, object, zookie, created_at)
VALUES ($3, $4, $1, $5, $6, $7, $8, (SELECT bump.revision_counter FROM bump), now())
ON CONFLICT (organization_id, namespace_id, subject, relation, object) DO NOTHING
RETURNING id, organization_id, namespace_id, subject, subject_relation, relation, object, zookie, created_at
`

const queryGetTupleByKey = `
SELECT t.id, t.organization_id, t.namespace_id, t.subject, t.subject_relation, t.relation, t.object,
       n.revision_counter::text, t.created_at
FROM guliangiam.relation_tuples t
JOIN guliangiam.relation_namespaces n
  ON n.id = t.namespace_id AND n.organization_id = t.organization_id
WHERE t.organization_id = $1 AND t.namespace_id = $2
  AND t.subject = $3 AND t.relation = $4 AND t.object = $5
`

const queryListTuples = `
SELECT id, organization_id, namespace_id, subject, subject_relation, relation, object, zookie, created_at
FROM guliangiam.relation_tuples
WHERE organization_id = $1 AND namespace_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4
`

const queryListTuplesCount = `
SELECT count(*) FROM guliangiam.relation_tuples WHERE organization_id = $1 AND namespace_id = $2
`

// queryDeleteTuple 与 queryCreateTuple 对称：删除 tuple 的同一语句内
// bump namespace 的 revision_counter（删除也是关系图变更，必须推进 zookie）。
// 未删除任何行时 UPDATE 无输入行，整条语句返回 0 行 → 上层映射 NotFound。
const queryDeleteTuple = `
WITH deleted AS (
  DELETE FROM guliangiam.relation_tuples
  WHERE id = $1 AND organization_id = $2
  RETURNING namespace_id, organization_id
)
UPDATE guliangiam.relation_namespaces n
SET revision_counter = revision_counter + 1
FROM deleted d
WHERE n.id = d.namespace_id AND n.organization_id = d.organization_id
RETURNING n.id
`

const queryDeleteTuplesByNamespace = `
DELETE FROM guliangiam.relation_tuples
WHERE namespace_id = $1 AND organization_id = $2
`

// ─── Evaluator 批量拉取 ────────────────────────────────────────────────────

// queryFetchAllTuples 拉取某 namespace 下全部元组，支持 atZookie 快照边界：
//   - atZookie = ”  → 不加过滤，返回全量（最新快照）
//   - atZookie != ” → WHERE zookie <= $3，只返回该 revision 之前的元组
//
// zookie 存的是字符串形式的 bigint（revision_counter），
// 字符串比较对等长数字串等价于数值比较。
const queryFetchAllTuples = `
SELECT id, organization_id, namespace_id, subject, subject_relation, relation, object, zookie, created_at
FROM guliangiam.relation_tuples
WHERE organization_id = $1 AND namespace_id = $2
  AND ($3 = '' OR zookie <= $3)
`
