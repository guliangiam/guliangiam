package payments

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::jsonb / $N::text   → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   $4::text IS NULL OR col=$4         → ? IS NULL OR col = ?（传 nil 或 string）
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   pgx.Query/QueryRow/Exec            → QueryContext/QueryRowContext/ExecContext
//   products jsonb                     → TEXT（JSON 字符串）

const btPayments = "`payments`"

// jsonNullLiteral 是 MySQL TEXT 列里 JSON null 的序列化文本。
const jsonNullLiteral = "null"

// queryGetPaymentByID 占位符：id(1)
const queryGetPaymentByID = `SELECT id, organization_id, name, display_name, provider, type, products, product_name, product_display_name, detail, currency, price, user_id, person_name, person_id_card, person_email, person_phone, invoice_type, invoice_title, invoice_tax_id, invoice_remark, invoice_url, order_id, out_order_id, pay_url, success_url, status, message, idempotency_key, created_at, updated_at, revision FROM ` + btPayments + ` WHERE id = ?`

// queryGetPaymentByIDForOrganization 占位符：id(1)、organization_id(2)。
const queryGetPaymentByIDForOrganization = `SELECT id, organization_id, name, display_name, provider, type, products, product_name, product_display_name, detail, currency, price, user_id, person_name, person_id_card, person_email, person_phone, invoice_type, invoice_title, invoice_tax_id, invoice_remark, invoice_url, order_id, out_order_id, pay_url, success_url, status, message, idempotency_key, created_at, updated_at, revision FROM ` + btPayments + ` WHERE id = ? AND organization_id = ?`

// queryListPayments 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%') → 3
//	user_id: CASE WHEN ?='' THEN TRUE ELSE user_id=?       → 2
//	status: ? IS NULL OR status=?                            → 2
//	limit, offset                                         → 2
//	总计：11
const queryListPayments = `SELECT id, organization_id, name, display_name, provider, type, products, product_name, product_display_name, detail, currency, price, user_id, person_name, person_id_card, person_email, person_phone, invoice_type, invoice_title, invoice_tax_id, invoice_remark, invoice_url, order_id, out_order_id, pay_url, success_url, status, message, idempotency_key, created_at, updated_at, revision FROM ` + btPayments + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (CASE WHEN ? = '' THEN TRUE ELSE user_id = ? END) AND (? IS NULL OR status = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountPayments: org_id×2 + keyword×3 + user_id×2 + status×2 = 9
const queryCountPayments = `SELECT COUNT(*) FROM ` + btPayments + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%')) AND (CASE WHEN ? = '' THEN TRUE ELSE user_id = ? END) AND (? IS NULL OR status = ?)`

const queryInsertPayment = `INSERT INTO ` + btPayments + ` (id, organization_id, name, display_name, provider, type, products, product_name, product_display_name, detail, currency, price, user_id, person_name, person_id_card, person_email, person_phone, invoice_type, invoice_title, invoice_tax_id, invoice_remark, invoice_url, order_id, out_order_id, pay_url, success_url, status, message, idempotency_key, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const queryUpdatePayment = `UPDATE ` + btPayments + ` SET display_name=?, provider=?, type=?, status=?, message=?, pay_url=?, success_url=?, out_order_id=?, invoice_url=?, idempotency_key=?, updated_at=?, revision=? WHERE id=? AND organization_id=?`

const queryDeletePayment = `DELETE FROM ` + btPayments + ` WHERE id = ? AND organization_id = ?`
