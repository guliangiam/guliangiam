package mysql

// ─── relation_namespaces ──────────────────────────────────────────────────────

// 注意：MySQL 用 `?` 作参数占位符（不同于 PG 的 `$1`, `$2`）。

const queryCreateNamespace = `
INSERT INTO relation_namespaces
  (id, organization_id, name, display_name, relation_defs, version, created_at, updated_at, revision_counter)
VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW(), 0)
`

const queryListNamespaces = `
SELECT id, organization_id, name, display_name, relation_defs, version, created_at, updated_at, revision_counter
FROM relation_namespaces
WHERE organization_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?
`

const queryListNamespacesCount = `
SELECT COUNT(*) FROM relation_namespaces WHERE organization_id = ?
`

const queryGetNamespace = `
SELECT id, organization_id, name, display_name, relation_defs, version, created_at, updated_at, revision_counter
FROM relation_namespaces
WHERE id = ? AND organization_id = ?
`

const queryGetNamespaceByName = `
SELECT id, organization_id, name, display_name, relation_defs, version, created_at, updated_at, revision_counter
FROM relation_namespaces
WHERE organization_id = ? AND name = ?
`

const queryDeleteNamespace = `
DELETE FROM relation_namespaces
WHERE id = ? AND organization_id = ?
`

const queryGetLatestZookie = `
SELECT revision_counter FROM relation_namespaces
WHERE id = ? AND organization_id = ?
`

// ─── relation_tuples ──────────────────────────────────────────────────────────
// MySQL 无 UPDATE ... RETURNING —— CreateTuple 必须 Go 层事务两步：
//   1. UPDATE relation_namespaces SET revision_counter = revision_counter + 1
//      WHERE id = ? AND organization_id = ?
//   2. SELECT revision_counter INTO @zookie WHERE ...
//   3. INSERT IGNORE relation_tuples (..., zookie) VALUES (..., @zookie)
//
// 在 Go 代码层包装成 *sql.Tx，保证原子性。

const queryBumpRevision = `
UPDATE relation_namespaces
SET revision_counter = revision_counter + 1
WHERE id = ? AND organization_id = ?
`

const queryReadNewRevision = `
SELECT revision_counter FROM relation_namespaces
WHERE id = ? AND organization_id = ?
`

const queryInsertTuple = `
INSERT IGNORE INTO relation_tuples
  (id, organization_id, namespace_id, subject, subject_relation, relation, object, zookie, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW())
`

const queryGetTupleByKey = `
SELECT t.id, t.organization_id, t.namespace_id, t.subject, t.subject_relation, t.relation, t.object,
       CAST(n.revision_counter AS CHAR), t.created_at
FROM relation_tuples t
JOIN relation_namespaces n
  ON n.id = t.namespace_id AND n.organization_id = t.organization_id
WHERE t.organization_id = ? AND t.namespace_id = ?
  AND t.subject = ? AND t.relation = ? AND t.object = ?
`

const queryListTuples = `
SELECT id, organization_id, namespace_id, subject, subject_relation, relation, object, zookie, created_at
FROM relation_tuples
WHERE organization_id = ? AND namespace_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?
`

const queryListTuplesCount = `
SELECT COUNT(*) FROM relation_tuples WHERE organization_id = ? AND namespace_id = ?
`

const queryDeleteTuple = `
DELETE FROM relation_tuples
WHERE id = ? AND organization_id = ?
`

const queryDeleteTuplesByNamespace = `
DELETE FROM relation_tuples
WHERE namespace_id = ? AND organization_id = ?
`

// ─── Evaluator 批量拉取 ───────────────────────────────────────────────────────

// queryFetchAllTuples 与 PG 版语义完全一致。MySQL 8.4 用
// ($3 = ” OR zookie <= $3) 占位符的位置同样从 1 开始。
// zookie 存的是数字串，字符串比较对同构值等价于数值比较。
const queryFetchAllTuples = `
SELECT id, organization_id, namespace_id, subject, subject_relation, relation, object, zookie, created_at
FROM relation_tuples
WHERE organization_id = ? AND namespace_id = ?
  AND (? = '' OR zookie <= ?)
`
