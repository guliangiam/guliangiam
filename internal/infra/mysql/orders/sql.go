package orders

// SQL 方言转换规则（PG → MySQL）：
//   $N          → ?
//   guliangiam.t → `t`
//   ILIKE       → LIKE
//   a || b      → CONCAT(a, b)
//   ::uuid / ::jsonb / ::text cast → 删除
//   resolve_org_id(text) → 删除（直接传 organization_id 做 CASE WHEN 判断）
//   $4::text IS NULL → ? IS NULL

const btOrders = "`orders`"

// queryGetOrderByID 按主键取订单（系统作用域；HTTP 管理路由必须用 queryGetOrderByIDForOrganization）。
// 参数：1 个 ? = order id
const queryGetOrderByID = `
SELECT id, organization_id, name, display_name, user_id, products, product_infos, payment, price, currency, state, message, payment_url, trade_no, coupon_name, coupon_discount, idempotency_key, created_at, updated_at, paid_at, revision
FROM ` + btOrders + `
WHERE id = ?`

// queryGetOrderByIDForOrganization 按 id + organization_id 取订单；跨租户与不存在统一 NotFound。
// 参数：2 个 ? = order id, organization_id
const queryGetOrderByIDForOrganization = `
SELECT id, organization_id, name, display_name, user_id, products, product_infos, payment, price, currency, state, message, payment_url, trade_no, coupon_name, coupon_discount, idempotency_key, created_at, updated_at, paid_at, revision
FROM ` + btOrders + `
WHERE id = ? AND organization_id = ?`

// queryGetOrderByTradeNo 按 trade_no 取订单。
// 参数：1 个 ? = trade_no
const queryGetOrderByTradeNo = `
SELECT id, organization_id, name, display_name, user_id, products, product_infos, payment, price, currency, state, message, payment_url, trade_no, coupon_name, coupon_discount, idempotency_key, created_at, updated_at, paid_at, revision
FROM ` + btOrders + `
WHERE trade_no = ?`

// queryListOrders 分页列出订单。
// MySQL 版参数顺序（共 11 个）：
//
//	org_id×2, keyword×3, user×2, state×2, limit, offset
//	- org_id: CASE WHEN + 实际比较
//	- keyword: CASE WHEN + 2 个 LIKE CONCAT
//	- user: CASE WHEN + 实际比较
//	- state: IS NULL OR state = state
const queryListOrders = `
SELECT id, organization_id, name, display_name, user_id, products, product_infos, payment, price, currency, state, message, payment_url, trade_no, coupon_name, coupon_discount, idempotency_key, created_at, updated_at, paid_at, revision
FROM ` + btOrders + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (CASE WHEN ? = '' THEN TRUE ELSE user_id = ? END)
  AND (? IS NULL OR state = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCountOrders 与 List 条件同构，只返回 COUNT。
// 参数顺序：org_id×2, keyword×3, user×2, state×2 = 9
const queryCountOrders = `
SELECT COUNT(*)
FROM ` + btOrders + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (CASE WHEN ? = '' THEN TRUE ELSE user_id = ? END)
  AND (? IS NULL OR state = ?)`

// queryInsertOrder 插入订单。
// 参数顺序：与 INSERT 列严格对齐，21 个 ?。
const queryInsertOrder = `
INSERT INTO ` + btOrders + ` (id, organization_id, name, display_name, user_id, products, product_infos, payment, price, currency, state, message, payment_url, trade_no, coupon_name, coupon_discount, idempotency_key, created_at, updated_at, paid_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateOrder 更新订单（带乐观锁 revision + 租户谓词）。
// SET 子句 14 列 + WHERE id + WHERE old_revision + WHERE organization_id = 17 个 ?
const queryUpdateOrder = `
UPDATE ` + btOrders + `
SET display_name = ?,
    user_id = ?,
    payment = ?,
    price = ?,
    currency = ?,
    state = ?,
    message = ?,
    payment_url = ?,
    trade_no = ?,
    coupon_name = ?,
    coupon_discount = ?,
    idempotency_key = ?,
    updated_at = ?,
    paid_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

// querySelectOrderRevision 回读 revision（UPDATE 后执行，带租户谓词）
const querySelectOrderRevision = `SELECT revision FROM ` + btOrders + ` WHERE id = ? AND organization_id = ?`

// queryDeleteOrder 按主键 + 租户删除订单。
// 参数：2 个 ? = order id, organization_id
const queryDeleteOrder = `DELETE FROM ` + btOrders + ` WHERE id = ? AND organization_id = ?`
