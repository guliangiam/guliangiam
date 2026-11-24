package subscriptions

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::text                → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   $4::text IS NULL OR state = $4     → ? = '' OR state = ?（传两次）
//   now()                              → NOW()

const btSubscriptions = "`subscriptions`"

// queryGetByIDForOrganization: id(1), organization_id(2) — 跨租户与不存在统一 NotFound
const queryGetByIDForOrganization = `
SELECT id, organization_id, name, display_name, user_id, plan_id, pricing_id, state,
       start_at, end_at, expire_at, created_at, updated_at, revision
FROM ` + btSubscriptions + `
WHERE id = ? AND organization_id = ?`

// queryListSubscriptions
//
//	org_id: CASE WHEN ?='' THEN TRUE ELSE organization_id=? END    → 2
//	keyword: ?='' OR name LIKE CONCAT('%',?, '%') OR display_name LIKE CONCAT('%',?, '%') → 3
//	user: CASE WHEN ?='' THEN TRUE ELSE user_id=? END               → 2
//	state: ?='' OR state=?                                          → 2
//	limit, offset                                                   → 2
//	总计：11
const queryListSubscriptions = `
SELECT id, organization_id, name, display_name, user_id, plan_id, pricing_id, state,
       start_at, end_at, expire_at, created_at, updated_at, revision
FROM ` + btSubscriptions + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (CASE WHEN ? = '' THEN TRUE ELSE user_id = ? END)
  AND (? = '' OR state = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCountSubscriptions: org×2 + keyword×3 + user×2 + state×2 = 9
const queryCountSubscriptions = `
SELECT COUNT(*)
FROM ` + btSubscriptions + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (CASE WHEN ? = '' THEN TRUE ELSE user_id = ? END)
  AND (? = '' OR state = ?)`

// queryInsertSubscription: 14 个占位符
const queryInsertSubscription = `
INSERT INTO ` + btSubscriptions + ` (id, organization_id, name, display_name, user_id, plan_id, pricing_id, state, start_at, end_at, expire_at, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateSubscription: display_name(1), state(1), start_at(1), end_at(1), expire_at(1), updated_at(1), id(1), organization_id(1) → 8
const queryUpdateSubscription = `
UPDATE ` + btSubscriptions + `
SET display_name = ?, state = ?, start_at = ?, end_at = ?, expire_at = ?, updated_at = ?
WHERE id = ? AND organization_id = ?`

// queryDeleteSubscription: id(1), organization_id(1)
const queryDeleteSubscription = `
DELETE FROM ` + btSubscriptions + `
WHERE id = ? AND organization_id = ?`
