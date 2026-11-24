package products

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::jsonb              → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   $3::text IS NULL OR col=$3         → ? IS NULL OR col = ?（直接传 nil 或 string）
//   $4::boolean IS NULL OR col=$4      → ? IS NULL OR col = ?（直接传 nil 或 bool）

const btProducts = "`products`"

// queryGetProductByID 是结算/购买路径使用的系统作用域查询（占位符：id(1)）。
const queryGetProductByID = `SELECT id, organization_id, name, display_name, image, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision FROM ` + btProducts + ` WHERE id = ?`

// queryGetProductByIDForOrganization 占位符：id(1)、organization_id(2)
const queryGetProductByIDForOrganization = `SELECT id, organization_id, name, display_name, image, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision FROM ` + btProducts + ` WHERE id = ? AND organization_id = ?`

// queryListProducts 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') → 3
//	status: ? IS NULL OR status=?                           → 2
//	is_recharge: ? IS NULL OR is_recharge=?                → 2
//	limit, offset                                         → 2
//	总计：11
const queryListProducts = `SELECT id, organization_id, name, display_name, image, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision FROM ` + btProducts + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR status = ?) AND (? IS NULL OR is_recharge = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountProducts: org_id×2 + keyword×3 + status×2 + is_recharge×2 = 9
const queryCountProducts = `SELECT COUNT(*) FROM ` + btProducts + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (? IS NULL OR status = ?) AND (? IS NULL OR is_recharge = ?)`

const queryInsertProduct = `INSERT INTO ` + btProducts + ` (id, organization_id, name, display_name, image, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateProduct 占位符：SET 17 + id + organization_id = 19；organization_id 取自实体。
const queryUpdateProduct = `UPDATE ` + btProducts + ` SET display_name=?, image=?, detail=?, description=?, tag=?, currency=?, price=?, quantity=?, sold=?, is_recharge=?, recharge_options=?, disable_custom_recharge=?, providers=?, success_url=?, status=?, updated_at=?, revision=? WHERE id=? AND organization_id=?`

// queryDeleteProduct 占位符：id(1)、organization_id(2)
const queryDeleteProduct = `DELETE FROM ` + btProducts + ` WHERE id = ? AND organization_id = ?`
