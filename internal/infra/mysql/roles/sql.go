package roles

// SQL 方言转换规则（PG → MySQL）：
//
//	$N                      → ?
//	guliangiam.t            → `t`（Go raw string 不能含反引号，用字符串拼接）
//	ILIKE                   → LIKE
//	'a' || b || 'c'         → CONCAT('a', b, 'c')
//	now()                   → NOW()
//	ON CONFLICT DO UPDATE   → ON DUPLICATE KEY UPDATE
//	RETURNING               → 删除（Create 不回读；Update 改为普通 UPDATE + SELECT revision）
//	::uuid / ::jsonb cast   → 删除
//	resolve_org_id(text)    → 删除（MySQL 无此函数，repository 层直接传 organization_id）
//	$3::boolean IS NULL     → (? IS NULL)（*bool nil 时传 nil 到 sqlmock 不 compare）
//
// 表名常量（Go raw string 不能含反引号，用普通字符串拼接）。
const btRoles = "`roles`"

const (
	// queryGetRoleByIDForOrganization 按主键 + 租户取 role。
	// 参数：role id(1)、organization_id(2)
	queryGetRoleByIDForOrganization = `
SELECT id, organization_id, name, display_name, description, COALESCE(users, '[]') AS users, group_ids, role_ids, domains, is_enabled, created_at, updated_at, revision
FROM ` + btRoles + `
WHERE id = ? AND organization_id = ?`

	// queryListRoles 分页列出 roles。
	// 方言替换：
	//   PG CASE WHEN $1 = '' → MySQL (? = '' OR organization_id = ?)
	//   PG $2 出现 1 次 → MySQL keyword 出现 4 次（跳过条件 + 3 个 LIKE）
	//   PG $3::boolean IS NULL → MySQL (? IS NULL)
	//
	// 参数顺序：organization_id(×2), keyword(×4), is_enabled(×2), limit, offset（共 10 个）
	queryListRoles = `
SELECT id, organization_id, name, display_name, description, COALESCE(users, '[]') AS users, group_ids, role_ids, domains, is_enabled, created_at, updated_at, revision
FROM ` + btRoles + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') OR description LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	// queryCountRoles 与 List 条件同构，只返回 COUNT。
	// 参数顺序：organization_id(×2), keyword(×4), is_enabled(×2)（共 8 个）
	queryCountRoles = `
SELECT COUNT(*)
FROM ` + btRoles + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') OR description LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)`

	// queryInsertRole 插入 role。
	// 参数：id, organization_id, name, display_name, description, users, group_ids, role_ids, domains, is_enabled, created_at, updated_at, revision（共 13 个）
	queryInsertRole = `
INSERT INTO ` + btRoles + ` (id, organization_id, name, display_name, description, users, group_ids, role_ids, domains, is_enabled, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdateRole 乐观锁 + 租户谓词（organization_id 取自实体）。
	// 参数顺序：display_name, description, users, group_ids, role_ids, domains, is_enabled, updated_at, id, old_revision, organization_id（共 11 个）
	queryUpdateRole = `
UPDATE ` + btRoles + `
SET display_name = ?,
    description = ?,
    users = ?,
    group_ids = ?,
    role_ids = ?,
    domains = ?,
    is_enabled = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

	// querySelectRevision 回读 revision（UPDATE 后执行，替代 PG RETURNING）。
	// 参数：role id(1)、organization_id(2)
	querySelectRevision = `SELECT revision FROM ` + btRoles + ` WHERE id = ? AND organization_id = ?`

	// queryDeleteRole 删除 role；跨租户按不存在处理。
	// 参数：role id(1)、organization_id(2)
	queryDeleteRole = `DELETE FROM ` + btRoles + ` WHERE id = ? AND organization_id = ?`
)
