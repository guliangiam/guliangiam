package groups

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid                         → ?
//   "guliangiam".table               → `table`
//   ILIKE '%' || $2 || '%'           → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)    → 直接 organization_id = ?
//   $12::jsonb                       → MySQL TEXT (JSON 字符串)
//   RETURNING revision               → MySQL UPDATE + SELECT revision 两步
//   is_enabled = CASE WHEN $3 = 'active' ... → MySQL 保留原 CASE

const btGroups = "`groups`"

// queryGetGroupByID: id(1)
const queryGetGroupByID = `
SELECT id, organization_id, name, display_name, manager, contact_email, type, parent_id, is_top_group, is_enabled,
       COALESCE(homepage, '') AS homepage, COALESCE(users, '[]') AS users,
       created_at, updated_at, revision
FROM ` + btGroups + `
WHERE id = ?`

// queryGetGroupByIDForOrganization: id(1)、organization_id(2)
const queryGetGroupByIDForOrganization = `
SELECT id, organization_id, name, display_name, manager, contact_email, type, parent_id, is_top_group, is_enabled,
       COALESCE(homepage, '') AS homepage, COALESCE(users, '[]') AS users,
       created_at, updated_at, revision
FROM ` + btGroups + `
WHERE id = ? AND organization_id = ?`

// queryListGroups
//
//	org: CASE WHEN ?='' THEN TRUE ELSE organization_id=? END → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') → 3
//	status: ?='' OR is_enabled = CASE WHEN ?='active' THEN TRUE WHEN ?='inactive' THEN FALSE ELSE is_enabled END → 2
//	type: ?='' OR type=? → 2
//	ORDER BY（白名单解析）与 LIMIT ? OFFSET ? 由 List 拼接 → 2
//	总计：11
const queryListGroups = `
SELECT id, organization_id, name, display_name, manager, contact_email, type, parent_id, is_top_group, is_enabled,
       COALESCE(homepage, '') AS homepage, COALESCE(users, '[]') AS users,
       created_at, updated_at, revision
FROM ` + btGroups + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR is_enabled = CASE WHEN ? = 'active' THEN TRUE WHEN ? = 'inactive' THEN FALSE ELSE is_enabled END)
  AND (? = '' OR type = ?)`

// queryCountGroups: org×2 + keyword×3 + status×2 + type×2 = 9
const queryCountGroups = `
SELECT COUNT(*)
FROM ` + btGroups + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR is_enabled = CASE WHEN ? = 'active' THEN TRUE WHEN ? = 'inactive' THEN FALSE ELSE is_enabled END)
  AND (? = '' OR type = ?)`

// queryInsertGroup: 15 个占位符（users 用 JSON 字符串）
const queryInsertGroup = `
INSERT INTO ` + btGroups + ` (id, organization_id, name, display_name, manager, contact_email, type, parent_id, is_top_group, is_enabled, homepage, users, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateGroup: name(1), display_name(1), manager(1), contact_email(1), type(1), parent_id(1),
//
//	is_top_group(1), is_enabled(1), homepage(1), users(1), updated_at(1), id(1), revision(输入旧值)(1), organization_id(1) → 14
//
// WHERE revision=? 保证乐观锁；SET revision = revision + 1 自动递增
const queryUpdateGroup = `
UPDATE ` + btGroups + `
SET name = ?,
    display_name = ?,
    manager = ?,
    contact_email = ?,
    type = ?,
    parent_id = ?,
    is_top_group = ?,
    is_enabled = ?,
    homepage = ?,
    users = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

// querySelectGroupRevision: UPDATE 后取新 revision；id(1)、organization_id(2)
const querySelectGroupRevision = `SELECT revision FROM ` + btGroups + ` WHERE id = ? AND organization_id = ?`

// queryDeleteGroup: id(1)、organization_id(2)
const queryDeleteGroup = `
DELETE FROM ` + btGroups + `
WHERE id = ? AND organization_id = ?`

// queryCountChildrenByParentID: org(1), parent_id(1) → 2
const queryCountChildrenByParentID = `
SELECT COUNT(*)
FROM ` + btGroups + `
WHERE organization_id = ? AND parent_id = ?`
