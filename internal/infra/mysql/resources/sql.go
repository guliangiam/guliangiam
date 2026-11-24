package resources

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid                          → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   RETURNING revision                 → UPDATE + SELECT 两步

const btResources = "`resources`"

// queryGetResourceByID 占位符：id(1)、organization_id(2)
const queryGetResourceByID = `SELECT id, organization_id, application_id, name, display_name, type, tag, file_path, file_size, mime_type, created_at, updated_at, revision FROM ` + btResources + ` WHERE id = ? AND organization_id = ?`

// queryListResources 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	app_id: CASE WHEN ?='' → TRUE ELSE application_id=?      → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') → 3
//	type: ?='' OR type=?                                    → 2
//	tag: ?='' OR tag=?                                      → 2
//	limit, offset                                           → 2
//	总计：13
const queryListResources = `SELECT id, organization_id, application_id, name, display_name, type, tag, file_path, file_size, mime_type, created_at, updated_at, revision FROM ` + btResources + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (CASE WHEN ? = '' THEN TRUE ELSE application_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? = '' OR type = ?) AND (? = '' OR tag = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountResources: org_id×2 + app_id×2 + keyword×3 + type×2 + tag×2 = 11
const queryCountResources = `SELECT COUNT(*) FROM ` + btResources + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (CASE WHEN ? = '' THEN TRUE ELSE application_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? = '' OR type = ?) AND (? = '' OR tag = ?)`

const queryInsertResource = `INSERT INTO ` + btResources + ` (id, organization_id, application_id, name, display_name, type, tag, file_path, file_size, mime_type, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateResource: 乐观锁更新，revision 原子 +1，organization_id 取自实体。
const queryUpdateResource = `UPDATE ` + btResources + ` SET display_name = ?, type = ?, tag = ?, file_path = ?, file_size = ?, mime_type = ?, updated_at = ?, revision = revision + 1 WHERE id = ? AND revision = ? AND organization_id = ?`

// querySelectResourceRevision: UPDATE 后读新 revision（organization_id 谓词防止
// MySQL（无 RLS）在 id 复用时读到其他租户的行）
const querySelectResourceRevision = `SELECT revision FROM ` + btResources + ` WHERE id = ? AND organization_id = ?`

// queryDeleteResource 占位符：id(1)、organization_id(2)
const queryDeleteResource = `DELETE FROM ` + btResources + ` WHERE id = ? AND organization_id = ?`
