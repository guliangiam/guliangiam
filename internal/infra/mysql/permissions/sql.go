package permissions

// SQL 方言转换规则（PG → MySQL）：
//   $N          → ?
//   guliangiam.t → `t`（用 + "`t`" + ` 拼接，Go raw string 不能含反引号）
//   ILIKE       → LIKE
//   a || b      → CONCAT(a, b)
//   ::uuid / ::jsonb / ::boolean / ::text cast → 删除
//   $3::boolean IS NULL → (? IS NULL)
//   resolve_org_id(text) → 删除（MySQL 无此函数，repository 层直接传 organization_id）
//   RETURNING   → 删除（Update 改为普通 UPDATE + SELECT revision）

const (
	btPermissions = "`permissions`"
)

const (
	// queryGetPermissionByID 按主键取单条记录（系统级，无请求租户）。
	// 参数：1 个 ? = permission id
	queryGetPermissionByID = `
SELECT id, organization_id, name, display_name, description, users, groups, roles, domains, resource_type, resources, actions, effect, is_enabled, submitter, approver, state, COALESCE(model, '') AS model, COALESCE(adapter, '') AS adapter, COALESCE(sorter, '') AS sorter, COALESCE(application, '') AS application, approve_time, created_at, updated_at, revision
FROM ` + btPermissions + `
WHERE id = ?`

	// queryGetPermissionByIDForOrganization 按主键 + 组织取单条记录。
	// 参数：2 个 ? = permission id, organization_id
	queryGetPermissionByIDForOrganization = `
SELECT id, organization_id, name, display_name, description, users, groups, roles, domains, resource_type, resources, actions, effect, is_enabled, submitter, approver, state, COALESCE(model, '') AS model, COALESCE(adapter, '') AS adapter, COALESCE(sorter, '') AS sorter, COALESCE(application, '') AS application, approve_time, created_at, updated_at, revision
FROM ` + btPermissions + `
WHERE id = ? AND organization_id = ?`

	// queryListPermissions 分页列表。
	// 参数顺序：
	//   org_id × 2, keyword × 4, is_enabled × 2, state × 2, resource_type × 2, effect × 2, limit, offset = 16
	queryListPermissions = `
SELECT id, organization_id, name, display_name, description, users, groups, roles, domains, resource_type, resources, actions, effect, is_enabled, submitter, approver, state, COALESCE(model, '') AS model, COALESCE(adapter, '') AS adapter, COALESCE(sorter, '') AS sorter, COALESCE(application, '') AS application, approve_time, created_at, updated_at, revision
FROM ` + btPermissions + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') OR description LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
  AND (? = '' OR state = ?)
  AND (? = '' OR resource_type = ?)
  AND (? = '' OR effect = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	// queryCountPermissions 与 List 条件同构，只返回 COUNT。
	// 参数顺序：
	//   org_id × 2, keyword × 4, is_enabled × 2, state × 2, resource_type × 2, effect × 2 = 14
	queryCountPermissions = `
SELECT COUNT(*)
FROM ` + btPermissions + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') OR description LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
  AND (? = '' OR state = ?)
  AND (? = '' OR resource_type = ?)
  AND (? = '' OR effect = ?)`

	// queryInsertPermission 参数顺序：与 INSERT 列严格对齐，25 个 ?。
	queryInsertPermission = `
INSERT INTO ` + btPermissions + ` (id, organization_id, name, display_name, description, users, groups, roles, domains, resource_type, resources, actions, effect, is_enabled, submitter, approver, state, model, adapter, sorter, application, approve_time, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdatePermission 乐观锁更新（不带 RETURNING），带实体 organization_id 谓词。
	// SET 子句 20 列 + WHERE id + WHERE old_revision + WHERE organization_id = 23 个 ?
	queryUpdatePermission = `
UPDATE ` + btPermissions + `
SET display_name = ?,
    description = ?,
    users = ?,
    groups = ?,
    roles = ?,
    domains = ?,
    resource_type = ?,
    resources = ?,
    actions = ?,
    effect = ?,
    is_enabled = ?,
    submitter = ?,
    approver = ?,
    state = ?,
    model = ?,
    adapter = ?,
    sorter = ?,
    application = ?,
    approve_time = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

	// querySelectRevision 回读 revision（UPDATE 后执行），同样带组织谓词。
	querySelectRevision = `SELECT revision FROM ` + btPermissions + ` WHERE id = ? AND organization_id = ?`

	// queryDeletePermission 按主键 + 组织删除。
	queryDeletePermission = `DELETE FROM ` + btPermissions + ` WHERE id = ? AND organization_id = ?`
)
