package records

// SQL 方言转换规则（PG → MySQL）：
//   $N          → ?
//   guliangiam.t → `t`
//   ILIKE       → LIKE
//   a || b      → CONCAT(a, b)
//   ::uuid / ::text cast → 删除
//   resolve_org_id(text) → 删除（MySQL 无此函数，repository 层直接传 organization_id）
//   actor_id::text → actor_id（MySQL 直接 UUID/CHAR 比较）
//   CAST(id AS text) → CAST(id AS CHAR)

const btRecords = "`records`"

// recordsFilter 是四个 list/count 查询共享的 WHERE 谓词。
// MySQL 版：resolve_org_id 替换为直接 organization_id 比较；
// 每个占位符出现两次（CASE WHEN + 实际比较），所以每个过滤参数都需要传两次。
const recordsFilter = `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR actor_id = ?)
  AND (? = '' OR action = ?)
  AND (? = '' OR resource_type = ?)`

// recordsKeyword 是 list/search 查询共享的 LIKE 分支。
// keyword 参数出现 6 次：CASE WHEN + 5 个 LIKE CONCAT。
const recordsKeyword = `
  AND (? = '' OR CAST(id AS CHAR) LIKE CONCAT('%', ?, '%')
       OR actor_id LIKE CONCAT('%', ?, '%')
       OR action LIKE CONCAT('%', ?, '%')
       OR resource_type LIKE CONCAT('%', ?, '%')
       OR resource_id LIKE CONCAT('%', ?, '%'))`

// recordsTimeRange 是 search 查询可选的 [start, end] 时间窗口。
// start_time 和 end_time 各出现两次（IS NULL 检查 + 实际比较）。
const recordsTimeRange = `
  AND (? IS NULL OR created_at >= ?)
  AND (? IS NULL OR created_at <= ?)`

const queryCreateRecord = `
INSERT INTO ` + btRecords + ` (id, organization_id, actor_id, action, resource_type, resource_id, ip, user_agent, surface, request_id, trace_id, metadata, created_at, chain_id, chain_seq, prev_hash, entry_hash)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// List 查询的参数顺序（共 14 个）：
//
//	org_id×2, actor_id×2, action×2, resource_type×2, keyword×6, limit, offset
//
// Count 查询去掉 limit/offset = 12 个
var (
	queryListRecords = `
SELECT id, organization_id, actor_id, action, resource_type, resource_id, ip, user_agent, surface, request_id, trace_id, metadata, created_at
FROM ` + btRecords + recordsFilter + recordsKeyword + `
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	queryCountRecords = `
SELECT COUNT(*)
FROM ` + btRecords + recordsFilter + recordsKeyword

	// Search 查询参数顺序（共 22 个）：
	//   org_id×2, actor_id×2, action×2, resource_type×2, resource_id×2,
	//   start_time×2, end_time×2, keyword×6, limit, offset
	// Count 去掉 limit/offset = 20 个
	querySearchRecords = `
SELECT id, organization_id, actor_id, action, resource_type, resource_id, ip, user_agent, surface, request_id, trace_id, metadata, created_at
FROM ` + btRecords + recordsFilter + `
  AND (? = '' OR resource_id = ?)` + recordsTimeRange + recordsKeyword + `
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	queryCountSearchRecords = `
SELECT COUNT(*)
FROM ` + btRecords + recordsFilter + `
  AND (? = '' OR resource_id = ?)` + recordsTimeRange + recordsKeyword
)
