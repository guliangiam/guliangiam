package notificationproviders

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::boolean              → ?
//   guliangiam.table                   → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   $3::boolean IS NULL OR col=$3      → ? IS NULL OR col = ?（直接传 nil 或 bool）
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   RETURNING revision                  → UPDATE + SELECT revision 两步

const btNotif = "`notification_providers`"

// queryGetNotificationProviderByID 占位符：id(1)
const queryGetNotificationProviderByID = `SELECT id, organization_id, name, provider_type, webhook_url, access_token, config, is_enabled, created_at, updated_at, revision FROM ` + btNotif + ` WHERE id = ?`

// queryGetNotificationProviderByIDForOrganization 占位符：id(1)、organization_id(2)
const queryGetNotificationProviderByIDForOrganization = `SELECT id, organization_id, name, provider_type, webhook_url, access_token, config, is_enabled, created_at, updated_at, revision FROM ` + btNotif + ` WHERE id = ? AND organization_id = ?`

// queryListNotificationProviders 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR provider_type LIKE CONCAT('%', ?, '%') → 3
//	is_enabled: ? IS NULL OR is_enabled=?                 → 2
//	limit, offset                                         → 2
//	总计：9
const queryListNotificationProviders = `SELECT id, organization_id, name, provider_type, webhook_url, access_token, config, is_enabled, created_at, updated_at, revision FROM ` + btNotif + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR provider_type LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR is_enabled = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountNotificationProviders: org_id×2 + keyword×3 + is_enabled×2 = 7
const queryCountNotificationProviders = `SELECT COUNT(*) FROM ` + btNotif + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR provider_type LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR is_enabled = ?)`

// queryInsertNotificationProvider 占位符：12
const queryInsertNotificationProvider = `INSERT INTO ` + btNotif + ` (id, organization_id, provider_id, name, provider_type, webhook_url, access_token, config, is_enabled, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateNotificationProvider: SET ... revision = revision + 1 WHERE id=? AND revision=? AND organization_id=?
// 实体 OrganizationID 同时作为谓词；app 层已 scoped 读取并 Revision++，WHERE 用 Revision-1。
const queryUpdateNotificationProvider = `UPDATE ` + btNotif + ` SET name = ?, provider_type = ?, webhook_url = ?, access_token = ?, config = ?, is_enabled = ?, updated_at = ?, revision = revision + 1 WHERE id = ? AND revision = ? AND organization_id = ?`

// queryDeleteNotificationProvider 占位符：id(1)、organization_id(2)
const queryDeleteNotificationProvider = `DELETE FROM ` + btNotif + ` WHERE id = ? AND organization_id = ?`

// querySelectRevisionNotificationProvider UPDATE 后回读 revision（organization_id 谓词防止
// MySQL（无 RLS）在 id 复用时读到其他租户的行）
const querySelectRevisionNotificationProvider = `SELECT revision FROM ` + btNotif + ` WHERE id = ? AND organization_id = ?`
