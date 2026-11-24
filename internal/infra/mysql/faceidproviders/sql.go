package faceidproviders

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::boolean              → ?
//   guliangiam.table                   → `table`
//   $2::boolean IS NULL OR col=$2      → ? IS NULL OR col = ?（直接传 nil 或 bool）
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   RETURNING revision                  → UPDATE + SELECT revision 两步

const btFaceID = "`face_id_providers`"

// access_key_secret 沿用历史列名，但库内保存的是绑定 (table, column, id) 的
// v2 信封密文（A-6.1）；空串表示“未配置”，仓储在写入前封装、读取时解封。

// queryGetFaceIdProviderByID 占位符：id(1)、organization_id(2)
const queryGetFaceIdProviderByID = `SELECT id, organization_id, name, provider_type, access_key_id, access_key_secret, endpoint, config, is_enabled, created_at, updated_at, revision FROM ` + btFaceID + ` WHERE id = ? AND organization_id = ?`

// queryListFaceIdProviders 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	is_enabled: ? IS NULL OR is_enabled=?                 → 2
//	limit, offset                                         → 2
//	总计：6
const queryListFaceIdProviders = `SELECT id, organization_id, name, provider_type, access_key_id, access_key_secret, endpoint, config, is_enabled, created_at, updated_at, revision FROM ` + btFaceID + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? IS NULL OR is_enabled = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountFaceIdProviders: org_id×2 + is_enabled×2 = 4
const queryCountFaceIdProviders = `SELECT COUNT(*) FROM ` + btFaceID + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? IS NULL OR is_enabled = ?)`

// queryInsertFaceIdProvider 占位符：12
const queryInsertFaceIdProvider = `INSERT INTO ` + btFaceID + ` (id, organization_id, name, provider_type, access_key_id, access_key_secret, endpoint, config, is_enabled, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateFaceIdProvider: SET ... revision = revision + 1 WHERE id=? AND revision=? AND organization_id=?
const queryUpdateFaceIdProvider = `UPDATE ` + btFaceID + ` SET name = ?, provider_type = ?, access_key_id = ?, access_key_secret = ?, endpoint = ?, config = ?, is_enabled = ?, updated_at = ?, revision = revision + 1 WHERE id = ? AND revision = ? AND organization_id = ?`

// queryDeleteFaceIdProvider 占位符：id(1)、organization_id(2)
const queryDeleteFaceIdProvider = `DELETE FROM ` + btFaceID + ` WHERE id = ? AND organization_id = ?`

// querySelectRevisionFaceIdProvider UPDATE 后回读 revision（organization_id 谓词防止
// MySQL（无 RLS）在 id 复用时读到其他租户的行）
const querySelectRevisionFaceIdProvider = `SELECT revision FROM ` + btFaceID + ` WHERE id = ? AND organization_id = ?`
