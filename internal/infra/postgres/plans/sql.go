package plans

const (
	// QueryGetByID — 占位符：id(1)、organization_id(2)。跨租户 id 与不存在同为 NotFound。
	QueryGetByID = `SELECT id, organization_id, name, display_name, description, price, currency, period, product, payment_providers, status, is_exclusive, role, created_at, updated_at, revision FROM "guliangiam".plans WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList    = `SELECT id, organization_id, name, display_name, description, price, currency, period, product, payment_providers, status, is_exclusive, role, created_at, updated_at, revision FROM "guliangiam".plans WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3 = '' OR status = $3) AND ($4 = '' OR period = $4) AND ($5::boolean IS NULL OR is_exclusive = $5) ORDER BY created_at DESC LIMIT $6 OFFSET $7`
	QueryCount   = `SELECT COUNT(*) FROM "guliangiam".plans WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3 = '' OR status = $3) AND ($4 = '' OR period = $4) AND ($5::boolean IS NULL OR is_exclusive = $5)`
	QueryInsert  = `INSERT INTO "guliangiam".plans (id, organization_id, name, display_name, description, price, currency, period, product, payment_providers, status, is_exclusive, role, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13, $14, $15, $16)`
	// QueryUpdate — organization_id 取自实体（$14）；调用方必须先做 scoped
	// GetByID，因此跨租户 id 不会到达 UPDATE。0 行表示行已消失/租户不符 → NotFound。
	QueryUpdate = `UPDATE "guliangiam".plans SET display_name=$1, description=$2, price=$3, currency=$4, period=$5, product=$6, payment_providers=$7::jsonb, status=$8, is_exclusive=$9, role=$10, updated_at=$11, revision=$12 WHERE id=$13 AND organization_id = guliangiam.resolve_org_id($14)`
	QueryDelete = `DELETE FROM "guliangiam".plans WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
