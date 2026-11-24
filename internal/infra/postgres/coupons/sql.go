package coupons

const (
	QueryGetByID = `SELECT id, organization_id, name, display_name, description, code, discount_type, discount, max_discount, scope, products, users, quantity, used_count, max_usage_per_user, start_time, expire_time, min_order_amount, currency, status, created_at, updated_at, revision FROM "guliangiam".coupons WHERE id = $1::uuid`
	// QueryGetByIDForOrganization is the management by-ID lookup: a foreign
	// or missing row is indistinguishable from not found.
	QueryGetByIDForOrganization = `SELECT id, organization_id, name, display_name, description, code, discount_type, discount, max_discount, scope, products, users, quantity, used_count, max_usage_per_user, start_time, expire_time, min_order_amount, currency, status, created_at, updated_at, revision FROM "guliangiam".coupons WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList                   = `SELECT id, organization_id, name, display_name, code, discount_type, discount, status, created_at, updated_at FROM "guliangiam".coupons WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%' OR code ILIKE '%' || $2 || '%') AND ($3::text IS NULL OR status = $3) AND ($4 = '' OR discount_type = $4) ORDER BY created_at DESC LIMIT $5 OFFSET $6`
	QueryCount                  = `SELECT COUNT(*) FROM "guliangiam".coupons WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%' OR code ILIKE '%' || $2 || '%') AND ($3::text IS NULL OR status = $3) AND ($4 = '' OR discount_type = $4)`
	QueryInsert                 = `INSERT INTO "guliangiam".coupons (id, organization_id, name, display_name, description, code, discount_type, discount, max_discount, scope, products, users, quantity, used_count, max_usage_per_user, start_time, expire_time, min_order_amount, currency, status, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12::jsonb, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)`
	// QueryUpdate — SET 16 + id + entity organization_id($18) + expected
	// revision($19). The app layer reads the row scoped first, so 0 affected
	// rows means the row disappeared or was concurrently modified between the
	// read and the write; the repository re-reads to tell NotFound from Conflict.
	QueryUpdate = `UPDATE "guliangiam".coupons SET display_name=$1, description=$2, discount=$3, max_discount=$4, scope=$5, products=$6::jsonb, users=$7::jsonb, quantity=$8, max_usage_per_user=$9, start_time=$10, expire_time=$11, min_order_amount=$12, currency=$13, status=$14, updated_at=$15, revision=$16 WHERE id=$17 AND organization_id = guliangiam.resolve_org_id($18) AND revision=$19`
	QueryDelete = `DELETE FROM "guliangiam".coupons WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	// QueryGetByCode is the exact-code checkout lookup (BE-C-07): code equality,
	// never ILIKE, and always tenant-scoped.
	QueryGetByCode = `SELECT id, organization_id, name, display_name, description, code, discount_type, discount, max_discount, scope, products, users, quantity, used_count, max_usage_per_user, start_time, expire_time, min_order_amount, currency, status, created_at, updated_at, revision FROM "guliangiam".coupons WHERE organization_id = guliangiam.resolve_org_id($1) AND code = $2`
	// QueryIncrementUsage atomically consumes one coupon use (BE-C-06):
	// used_count is incremented in SQL (never read-modify-write), guarded by
	// the active state, the total quantity and the per-user usage limit, and
	// pinned to the revision the caller read. RETURNING yields the fresh row.
	QueryIncrementUsage = `UPDATE "guliangiam".coupons SET used_count = used_count + 1, updated_at = $3, revision = revision + 1 WHERE id = $1::uuid AND status = 'active' AND revision = $4 AND (quantity <= 0 OR used_count < quantity) AND (max_usage_per_user <= 0 OR (SELECT COUNT(*) FROM "guliangiam".coupon_usage WHERE coupon_id = $1::uuid AND user_id = NULLIF($2, '')::uuid) < max_usage_per_user) RETURNING id, organization_id, name, display_name, description, code, discount_type, discount, max_discount, scope, products, users, quantity, used_count, max_usage_per_user, start_time, expire_time, min_order_amount, currency, status, created_at, updated_at, revision`
)
