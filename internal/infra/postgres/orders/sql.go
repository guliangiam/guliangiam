package orders

const (
	// QueryGetByID — 系统作用域；HTTP 管理路由必须用 QueryGetByIDForOrganization。
	QueryGetByID = `SELECT id, organization_id, name, display_name, user_id, products, product_infos, payment, price, currency, status, message, payment_url, trade_no, coupon_name, coupon_discount, idempotency_key, created_at, updated_at, paid_at, revision FROM "guliangiam".orders WHERE id = $1::uuid`
	// QueryGetByIDForOrganization — 占位符：id(1)、organization_id(2)。
	QueryGetByIDForOrganization = `SELECT id, organization_id, name, display_name, user_id, products, product_infos, payment, price, currency, status, message, payment_url, trade_no, coupon_name, coupon_discount, idempotency_key, created_at, updated_at, paid_at, revision FROM "guliangiam".orders WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryGetByTradeNo           = `SELECT id, organization_id, name, display_name, user_id, products, product_infos, payment, price, currency, status, message, payment_url, trade_no, coupon_name, coupon_discount, idempotency_key, created_at, updated_at, paid_at, revision FROM "guliangiam".orders WHERE trade_no = $1`
	QueryList                   = `SELECT id, organization_id, name, display_name, user_id, products, product_infos, payment, price, currency, status, message, payment_url, trade_no, coupon_name, coupon_discount, idempotency_key, created_at, updated_at, paid_at, revision FROM "guliangiam".orders WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND (CASE WHEN $3 = '' THEN TRUE ELSE user_id = $3::uuid END) AND ($4::text IS NULL OR status = $4) ORDER BY created_at DESC LIMIT $5 OFFSET $6`
	QueryCount                  = `SELECT COUNT(*) FROM "guliangiam".orders WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND (CASE WHEN $3 = '' THEN TRUE ELSE user_id = $3::uuid END) AND ($4::text IS NULL OR status = $4)`
	QueryInsert                 = `INSERT INTO "guliangiam".orders (id, organization_id, name, display_name, user_id, products, product_infos, payment, price, currency, status, message, payment_url, trade_no, coupon_name, coupon_discount, idempotency_key, created_at, updated_at, paid_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid, $6::jsonb, $7::jsonb, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`
	// QueryUpdate — 租户谓词取自实体 OrganizationID（$17）；调用方必须先做 scoped GetByID。
	QueryUpdate = `UPDATE "guliangiam".orders SET display_name=$1, user_id=$2, payment=$3, price=$4, currency=$5, status=$6, message=$7, payment_url=$8, trade_no=$9, coupon_name=$10, coupon_discount=$11, idempotency_key=$12, updated_at=$13, paid_at=$14, revision=$15 WHERE id=$16 AND organization_id=$17::uuid`
	// QueryDelete — 占位符：id(1)、organization_id(2)。
	QueryDelete = `DELETE FROM "guliangiam".orders WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
