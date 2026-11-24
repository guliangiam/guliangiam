package plans

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::jsonb              → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   $5::boolean IS NULL OR col=$5      → ? IS NULL OR col = ?（直接传 nil 或 bool）

const btPlans = "`plans`"

// queryGetPlanByID 占位符：id(1)、organization_id(2)
const queryGetPlanByID = `SELECT id, organization_id, name, display_name, description, price, currency, period, product, payment_providers, state, is_exclusive, role, created_at, updated_at, revision FROM ` + btPlans + ` WHERE id = ? AND organization_id = ?`

// queryListPlans 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') → 3
//	state: ?='' OR state=?                                → 2
//	period: ?='' OR period=?                              → 2
//	is_exclusive: ? IS NULL OR is_exclusive=?             → 2
//	limit, offset                                         → 2
//	总计：13
const queryListPlans = `SELECT id, organization_id, name, display_name, description, price, currency, period, product, payment_providers, state, is_exclusive, role, created_at, updated_at, revision FROM ` + btPlans + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? = '' OR state = ?) AND (? = '' OR period = ?) AND (? IS NULL OR is_exclusive = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountPlans: org_id×2 + keyword×3 + state×2 + period×2 + is_exclusive×2 = 11
const queryCountPlans = `SELECT COUNT(*) FROM ` + btPlans + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? = '' OR state = ?) AND (? = '' OR period = ?) AND (? IS NULL OR is_exclusive = ?)`

const queryInsertPlan = `INSERT INTO ` + btPlans + ` (id, organization_id, name, display_name, description, price, currency, period, product, payment_providers, state, is_exclusive, role, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdatePlan 占位符：SET 12 + id + organization_id = 14；organization_id 取自实体。
const queryUpdatePlan = `UPDATE ` + btPlans + ` SET display_name=?, description=?, price=?, currency=?, period=?, product=?, payment_providers=?, state=?, is_exclusive=?, role=?, updated_at=?, revision=? WHERE id=? AND organization_id=?`

// queryDeletePlan 占位符：id(1)、organization_id(2)
const queryDeletePlan = `DELETE FROM ` + btPlans + ` WHERE id = ? AND organization_id = ?`
