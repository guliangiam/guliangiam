package coupons

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::timestamptz / $N::text → ?
//   "guliangiam".table                   → `table`
//   guliangiam.resolve_org_id($1)        → 直接 organization_id = ?
//   ILIKE '%' || $2 || '%'               → LIKE CONCAT('%', ?, '%')
//   jsonb (products, users)              → TEXT (JSON 字符串)

const btCoupons = "`coupons`"
const btCouponUsage = "`coupon_usage`"

// ── Coupon Repository SQL ──────────────────────────────────────────

// couponSelectBase: 22 列
const couponSelectBase = `
SELECT id, organization_id, name, display_name, description, code,
       discount_type, discount, max_discount, scope, products, users,
       quantity, used_count, max_usage_per_user, start_time, expire_time,
       min_order_amount, currency, state, created_at, updated_at, revision
FROM ` + btCoupons

// queryGetByID: id(1)
const queryGetByID = couponSelectBase + `
WHERE id = ?`

// queryGetByIDForOrganization: id(1), organization_id(2)
const queryGetByIDForOrganization = couponSelectBase + `
WHERE id = ? AND organization_id = ?`

// queryList: org_id×2(2), keyword×3(3), state×2(2), discount_type×2(2), limit, offset(2) = 11
//
//	 CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END                     → 2
//	 AND (? = '' OR name LIKE CONCAT('%', ?, '%')
//	           OR display_name LIKE CONCAT('%', ?, '%')
//	           OR code LIKE CONCAT('%', ?, '%'))                                → 4
//	 AND (? IS NULL OR state = ?)                                               → 2
//	 AND (? = '' OR discount_type = ?)                                          → 2
//	 LIMIT ?, OFFSET ?                                                          → 2
//	总计 = 12
const queryList = couponSelectBase + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') OR code LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR state = ?)
  AND (? = '' OR discount_type = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCount: 去掉 LIMIT/OFFSET，占位符数 = 10
const queryCount = `
SELECT COUNT(*)
FROM ` + btCoupons + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') OR code LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR state = ?)
  AND (? = '' OR discount_type = ?)`

// queryInsert: 23 个占位符
const queryInsert = `
INSERT INTO ` + btCoupons + `
    (id, organization_id, name, display_name, description, code, discount_type, discount, max_discount, scope, products, users,
     quantity, used_count, max_usage_per_user, start_time, expire_time, min_order_amount, currency, state, created_at, updated_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdate: 16 个 SET 参数 + WHERE id = ? + organization_id = ? → 18
const queryUpdate = `
UPDATE ` + btCoupons + `
SET display_name = ?, description = ?, discount = ?, max_discount = ?, scope = ?,
    products = ?, users = ?, quantity = ?, max_usage_per_user = ?,
    start_time = ?, expire_time = ?, min_order_amount = ?, currency = ?,
    state = ?, updated_at = ?, revision = ?
WHERE id = ? AND organization_id = ?`

// queryDelete: id(1), organization_id(2)
const queryDelete = `
DELETE FROM ` + btCoupons + ` WHERE id = ? AND organization_id = ?`

// ── CouponUsageRepository SQL ──────────────────────────────────────

// queryUsageInsert: 6 个占位符
const queryUsageInsert = `
INSERT INTO ` + btCouponUsage + ` (id, coupon_id, user_id, order_id, used_at, created_at)
VALUES (?, ?, ?, ?, ?, ?)`

// queryUsageListByCoupon: coupon_id(1), page_size(1), offset(1) = 3
const queryUsageListByCoupon = `
SELECT id, coupon_id, user_id, order_id, used_at, created_at
FROM ` + btCouponUsage + `
WHERE coupon_id = ?
ORDER BY used_at DESC
LIMIT ? OFFSET ?`

// queryUsageCountByCoupon: coupon_id(1)
const queryUsageCountByCoupon = `
SELECT COUNT(*) FROM ` + btCouponUsage + ` WHERE coupon_id = ?`

// queryUsageCountByUser: coupon_id(1), user_id(1)
const queryUsageCountByUser = `
SELECT COUNT(*) FROM ` + btCouponUsage + ` WHERE coupon_id = ? AND user_id = ?`
