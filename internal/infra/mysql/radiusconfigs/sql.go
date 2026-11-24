package radiusconfigs

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::boolean              → ?
//   guliangiam.table                   → `table`
//   $2::boolean IS NULL OR col=$2      → ? IS NULL OR col = ?（直接传 nil 或 bool）
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   RETURNING revision                  → UPDATE + SELECT revision 两步

const btRadiusCfg = "`radius_configs`"

// queryGetRadiusConfigByID 占位符：id(1)、organization_id(2)
const queryGetRadiusConfigByID = `SELECT id, organization_id, name, host, port, shared_secret, auth_protocol, is_enabled, created_at, updated_at, revision FROM ` + btRadiusCfg + ` WHERE id = ? AND organization_id = ?`

// queryListRadiusConfigs 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	is_enabled: ? IS NULL OR is_enabled=?                 → 2
//	auth_protocol: ?='' OR auth_protocol=?                → 2
//	limit, offset                                         → 2
//	总计：8
const queryListRadiusConfigs = `SELECT id, organization_id, name, host, port, shared_secret, auth_protocol, is_enabled, created_at, updated_at, revision FROM ` + btRadiusCfg + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? IS NULL OR is_enabled = ?) AND (? = '' OR auth_protocol = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountRadiusConfigs: org_id×2 + is_enabled×2 + auth_protocol×2 = 6
const queryCountRadiusConfigs = `SELECT COUNT(*) FROM ` + btRadiusCfg + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? IS NULL OR is_enabled = ?) AND (? = '' OR auth_protocol = ?)`

// queryInsertRadiusConfig 占位符：11
const queryInsertRadiusConfig = `INSERT INTO ` + btRadiusCfg + ` (id, organization_id, name, host, port, shared_secret, auth_protocol, is_enabled, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateRadiusConfig: SET ... revision = revision + 1 WHERE id=? AND revision=? AND organization_id=?
const queryUpdateRadiusConfig = `UPDATE ` + btRadiusCfg + ` SET name = ?, host = ?, port = ?, shared_secret = ?, auth_protocol = ?, is_enabled = ?, updated_at = ?, revision = revision + 1 WHERE id = ? AND revision = ? AND organization_id = ?`

// queryDeleteRadiusConfig 占位符：id(1)、organization_id(2)
const queryDeleteRadiusConfig = `DELETE FROM ` + btRadiusCfg + ` WHERE id = ? AND organization_id = ?`

// querySelectRevisionRadiusConfig UPDATE 后回读 revision（organization_id 谓词防止
// MySQL（无 RLS）在 id 复用时读到其他租户的行）
const querySelectRevisionRadiusConfig = `SELECT revision FROM ` + btRadiusCfg + ` WHERE id = ? AND organization_id = ?`
