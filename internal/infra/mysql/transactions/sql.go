package transactions

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::text                → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   $5::text IS NULL OR status = $5    → ? = '' OR status = ?（传两次）
//   $6::text IS NULL OR transaction_type = $6 → ? = '' OR transaction_type = ?（传两次）
//   PG jsonb metadata                  → MySQL TEXT (JSON 字符串)
//   PG uuid nullable                   → MySQL *string (NULL 时传 nil)

const btTransactions = "`transactions`"

// queryGetByID: id(1) — 系统路径（退款/用户子资源）使用
const queryGetByID = `
SELECT id, organization_id, user_id, order_id, payment_id, transaction_type,
       amount, currency, status, provider_type, provider_id, metadata,
       created_at, updated_at, revision
FROM ` + btTransactions + `
WHERE id = ?`

// queryGetByIDForOrganization: id(1), organization_id(2) — 跨租户与不存在统一 NotFound
const queryGetByIDForOrganization = `
SELECT id, organization_id, user_id, order_id, payment_id, transaction_type,
       amount, currency, status, provider_type, provider_id, metadata,
       created_at, updated_at, revision
FROM ` + btTransactions + `
WHERE id = ? AND organization_id = ?`

// queryListTransactions
//
//	org: CASE WHEN ?='' THEN TRUE ELSE organization_id=? END                  → 2
//	keyword: ?='' OR transaction_type LIKE CONCAT('%',?, '%')               → 3
//	user: CASE WHEN ?='' THEN TRUE ELSE user_id=? END                        → 2
//	order: CASE WHEN ?='' THEN TRUE ELSE order_id=? END                      → 2
//	status: ?='' OR status=?                                                 → 2
//	type: ?='' OR transaction_type=?                                         → 2
//	limit, offset                                                            → 2
//	总计：15
const queryListTransactions = `
SELECT id, organization_id, user_id, order_id, payment_id, transaction_type,
       amount, currency, status, provider_type, provider_id, metadata,
       created_at, updated_at, revision
FROM ` + btTransactions + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR transaction_type LIKE CONCAT('%', ?, '%'))
  AND (CASE WHEN ? = '' THEN TRUE ELSE user_id = ? END)
  AND (CASE WHEN ? = '' THEN TRUE ELSE order_id = ? END)
  AND (? = '' OR status = ?)
  AND (? = '' OR transaction_type = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCountTransactions: org×2 + keyword×3 + user×2 + order×2 + status×2 + type×2 = 13
const queryCountTransactions = `
SELECT COUNT(*)
FROM ` + btTransactions + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR transaction_type LIKE CONCAT('%', ?, '%'))
  AND (CASE WHEN ? = '' THEN TRUE ELSE user_id = ? END)
  AND (CASE WHEN ? = '' THEN TRUE ELSE order_id = ? END)
  AND (? = '' OR status = ?)
  AND (? = '' OR transaction_type = ?)`

// queryInsertTransaction: 15 个占位符
// nullable UUID 字段传 nil 即可 → MySQL NULL
const queryInsertTransaction = `
INSERT INTO ` + btTransactions + ` (id, organization_id, user_id, order_id, payment_id, transaction_type, amount, currency, status, provider_type, provider_id, metadata, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateTransaction: status(1), metadata(1), updated_at(1), id(1), organization_id(1) → 5
const queryUpdateTransaction = `
UPDATE ` + btTransactions + `
SET status = ?, metadata = ?, updated_at = ?
WHERE id = ? AND organization_id = ?`

// queryDeleteTransaction: id(1), organization_id(1)
const queryDeleteTransaction = `
DELETE FROM ` + btTransactions + `
WHERE id = ? AND organization_id = ?`
