package products

const (
	// QueryGetByID is the system-scoped lookup used by checkout/purchase.
	QueryGetByID = `SELECT id, organization_id, name, display_name, image, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision FROM "guliangiam".products WHERE id = $1::uuid`
	// QueryGetByIDForOrganization — 占位符：id(1)、organization_id(2)。跨租户 id 与不存在同为 NotFound。
	QueryGetByIDForOrganization = `SELECT id, organization_id, name, display_name, image, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision FROM "guliangiam".products WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList                   = `SELECT id, organization_id, name, display_name, image, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision FROM "guliangiam".products WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::text IS NULL OR status = $3) AND ($4::boolean IS NULL OR is_recharge = $4) ORDER BY created_at DESC LIMIT $5 OFFSET $6`
	QueryCount                  = `SELECT COUNT(*) FROM "guliangiam".products WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::text IS NULL OR status = $3) AND ($4::boolean IS NULL OR is_recharge = $4)`
	QueryInsert                 = `INSERT INTO "guliangiam".products (id, organization_id, name, display_name, image, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15, $16::jsonb, $17, $18, $19, $20, $21)`
	// QueryUpdate — organization_id 取自实体（$19）；调用方必须先做 scoped
	// GetByID，因此跨租户 id 不会到达 UPDATE。0 行表示行已消失/租户不符 → NotFound。
	QueryUpdate = `UPDATE "guliangiam".products SET display_name=$1, image=$2, detail=$3, description=$4, tag=$5, currency=$6, price=$7, quantity=$8, sold=$9, is_recharge=$10, recharge_options=$11::jsonb, disable_custom_recharge=$12, providers=$13::jsonb, success_url=$14, status=$15, updated_at=$16, revision=$17 WHERE id=$18 AND organization_id = guliangiam.resolve_org_id($19)`
	QueryDelete = `DELETE FROM "guliangiam".products WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	// QueryIncrementSold 原子扣减库存（BE-C-11）：sold 在 SQL 内递增并受
	// sold+q<=quantity 守卫，并发下单不可能超卖；quantity<=0 表示不限量。
	// 占位符：id(1)、organization_id(2)、quantity(3)、updated_at(4)。
	QueryIncrementSold = `UPDATE "guliangiam".products SET sold = sold + $3, updated_at = $4, revision = revision + 1 WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2) AND status = 'active' AND $3 > 0 AND (quantity <= 0 OR sold + $3 <= quantity) RETURNING id, organization_id, name, display_name, image, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision`
	// QueryReleaseSold 归还未支付订单预留的库存（取消/删除时），sold 不为负。
	// 占位符同 QueryIncrementSold。
	QueryReleaseSold = `UPDATE "guliangiam".products SET sold = GREATEST(sold - $3, 0), updated_at = $4, revision = revision + 1 WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2) AND $3 > 0 RETURNING id, organization_id, name, display_name, image, detail, description, tag, currency, price, quantity, sold, is_recharge, recharge_options, disable_custom_recharge, providers, success_url, status, created_at, updated_at, revision`
)
