package captchaproviders

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::boolean              → ?
//   guliangiam.table                   → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   $3::boolean IS NULL OR col=$3      → ? IS NULL OR col = ?（直接传 nil 或 bool）
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   RETURNING revision                  → UPDATE + SELECT revision 两步

const btCaptcha = "`captcha_providers`"

// queryGetCaptchaProviderByID 占位符：id(1)、organization_id(2)
const queryGetCaptchaProviderByID = `SELECT id, organization_id, name, provider_type, site_key, secret_key, config, is_enabled, created_at, updated_at, revision FROM ` + btCaptcha + ` WHERE id = ? AND organization_id = ?`

// queryListCaptchaProviders 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR provider_type LIKE CONCAT('%', ?, '%') → 3
//	is_enabled: ? IS NULL OR is_enabled=?                 → 2
//	limit, offset                                         → 2
//	总计：9
const queryListCaptchaProviders = `SELECT id, organization_id, name, provider_type, site_key, secret_key, config, is_enabled, created_at, updated_at, revision FROM ` + btCaptcha + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR provider_type LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR is_enabled = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountCaptchaProviders: org_id×2 + keyword×3 + is_enabled×2 = 7
const queryCountCaptchaProviders = `SELECT COUNT(*) FROM ` + btCaptcha + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR provider_type LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR is_enabled = ?)`

// queryInsertCaptchaProvider 占位符：11
const queryInsertCaptchaProvider = `INSERT INTO ` + btCaptcha + ` (id, organization_id, name, provider_type, site_key, secret_key, config, is_enabled, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateCaptchaProvider 占位符：name(1), provider_type(2), site_key(3), secret_key(4), config(5), is_enabled(6), updated_at(7), WHERE id(8), revision(9), organization_id(10)
// revision = revision + 1 在 SET 子句里
const queryUpdateCaptchaProvider = `UPDATE ` + btCaptcha + ` SET name = ?, provider_type = ?, site_key = ?, secret_key = ?, config = ?, is_enabled = ?, updated_at = ?, revision = revision + 1 WHERE id = ? AND revision = ? AND organization_id = ?`

// queryDeleteCaptchaProvider 占位符：id(1)、organization_id(2)
const queryDeleteCaptchaProvider = `DELETE FROM ` + btCaptcha + ` WHERE id = ? AND organization_id = ?`

// querySelectRevisionCaptchaProvider UPDATE 后回读 revision
const querySelectRevisionCaptchaProvider = `SELECT revision FROM ` + btCaptcha + ` WHERE id = ? AND organization_id = ?`
