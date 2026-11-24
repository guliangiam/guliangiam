package adapters

// SQL 方言转换规则（PG → MySQL）：
//
//	$N::uuid / $N::boolean / $N::text  → ?
//	"guliangiam".table                  → `table`（去掉 schema 前缀）
//	ILIKE                               → LIKE
//	'a' || b || 'c'                     → CONCAT('a', b, 'c')
//	$3::boolean IS NULL                 → (? IS NULL)
//	guliangiam.resolve_org_id($1)       → $1（repo 层直接传 organization_id）
//	RETURNING revision                  → 删除（Update 拆成 UPDATE + SELECT revision）
const btAdapters = "`adapters`"

const (
	// queryGetAdapterByID — 占位符：id(1)、organization_id(2)。
	queryGetAdapterByID = `
SELECT id, organization_id, name, display_name, description, type, server, database, table_name, username, password,
	owner, use_same_db, database_type, host, port, db_user, db_password, database_name, policy, is_enabled,
	created_at, updated_at, revision
FROM ` + btAdapters + ` WHERE id = ? AND organization_id = ?`

	// queryListAdapters — 分页列出 adapters。
	// 方言替换：
	//   PG CASE WHEN $1='' → (? = '' OR organization_id = ?)    orgID × 2
	//   PG '%'||$2||'%' ILIKE → LIKE CONCAT('%', ?, '%')        keyword × 3
	//   PG $3::boolean IS NULL → (? IS NULL)                    is_enabled × 2
	//   PG $4 = '' OR database_type = $4 → (? = '' OR database_type = ?)  database_type × 2
	// 参数顺序：organization_id(×2), keyword(×3), is_enabled(×2), database_type(×2), limit, offset = 2+3+2+2+2 = 11
	queryListAdapters = `
SELECT id, organization_id, name, display_name, description, type, server, database, table_name, username, password,
	owner, use_same_db, database_type, host, port, db_user, db_password, database_name, policy, is_enabled,
	created_at, updated_at, revision
FROM ` + btAdapters + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
  AND (? = '' OR database_type = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	// queryCountAdapters — 与 List 条件同构，只返回 COUNT。
	// 参数顺序：organization_id(×2), keyword(×3), is_enabled(×2), database_type(×2) = 2+3+2+2 = 9
	queryCountAdapters = `
SELECT COUNT(*) FROM ` + btAdapters + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
  AND (? = '' OR database_type = ?)`

	// queryInsertAdapter — 插入 adapter。24 个 ?
	queryInsertAdapter = `
INSERT INTO ` + btAdapters + ` (
	id, organization_id, name, display_name, description, type, server, database, table_name, username, password,
	owner, use_same_db, database_type, host, port, db_user, db_password, database_name, policy, is_enabled,
	created_at, updated_at, revision
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdateAdapter — 乐观锁 + 租户谓词（organization_id 取自实体）。
	// 参数顺序：display_name, description, type, server, database, table_name, username, password,
	//           owner, use_same_db, database_type, host, port, db_user, db_password, database_name, policy, is_enabled,
	//           updated_at, id, revision, organization_id
	// 共 22 个（SET 19 + WHERE id + WHERE revision + WHERE organization_id）
	// 0 行 = revision 冲突（跨租户在 app 层 scoped GetByID 已拒绝，不产生租户探测差异）。
	queryUpdateAdapter = `
UPDATE ` + btAdapters + `
SET display_name = ?, description = ?, type = ?, server = ?, database = ?, table_name = ?, username = ?, password = ?,
    owner = ?, use_same_db = ?, database_type = ?, host = ?, port = ?, db_user = ?, db_password = ?, database_name = ?, policy = ?, is_enabled = ?,
    updated_at = ?, revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

	// querySelectRevision — 回读 revision（UPDATE 后执行，替代 PG RETURNING）；占位符：id(1)、organization_id(2)。
	querySelectRevision = `SELECT revision FROM ` + btAdapters + ` WHERE id = ? AND organization_id = ?`

	queryDeleteAdapter = `DELETE FROM ` + btAdapters + ` WHERE id = ? AND organization_id = ?`
)
