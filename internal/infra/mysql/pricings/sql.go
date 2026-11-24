package pricings

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::jsonb              → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   $3::boolean IS NULL OR col=$3      → ? IS NULL OR col = ?（直接传 nil 或 bool）

const btPricings = "`pricings`"

// queryGetPricingByID 占位符：id(1)、organization_id(2)
const queryGetPricingByID = `SELECT id, organization_id, name, display_name, description, plans, is_enabled, trial_duration, application, created_at, updated_at, revision FROM ` + btPricings + ` WHERE id = ? AND organization_id = ?`

// queryListPricings 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') → 3
//	is_enabled: ? IS NULL OR is_enabled=?                 → 2
//	limit, offset                                         → 2
//	总计：9
const queryListPricings = `SELECT id, organization_id, name, display_name, description, plans, is_enabled, trial_duration, application, created_at, updated_at, revision FROM ` + btPricings + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR is_enabled = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountPricings: org_id×2 + keyword×3 + is_enabled×2 = 7
const queryCountPricings = `SELECT COUNT(*) FROM ` + btPricings + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR is_enabled = ?)`

const queryInsertPricing = `INSERT INTO ` + btPricings + ` (id, organization_id, name, display_name, description, plans, is_enabled, trial_duration, application, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdatePricing 占位符：SET 8 + id + organization_id = 10；organization_id 取自实体。
const queryUpdatePricing = `UPDATE ` + btPricings + ` SET display_name=?, description=?, plans=?, is_enabled=?, trial_duration=?, application=?, updated_at=?, revision=? WHERE id=? AND organization_id=?`

// queryDeletePricing 占位符：id(1)、organization_id(2)
const queryDeletePricing = `DELETE FROM ` + btPricings + ` WHERE id = ? AND organization_id = ?`
