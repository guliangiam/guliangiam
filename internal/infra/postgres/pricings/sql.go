package pricings

const (
	// QueryGetByID — 占位符：id(1)、organization_id(2)。跨租户 id 与不存在同为 NotFound。
	QueryGetByID = `SELECT id, organization_id, name, display_name, description, plans, is_enabled, trial_duration, application, created_at, updated_at, revision FROM "guliangiam".pricings WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList    = `SELECT id, organization_id, name, display_name, description, plans, is_enabled, trial_duration, application, created_at, updated_at, revision FROM "guliangiam".pricings WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::boolean IS NULL OR is_enabled = $3) ORDER BY created_at DESC LIMIT $4 OFFSET $5`
	QueryCount   = `SELECT COUNT(*) FROM "guliangiam".pricings WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::boolean IS NULL OR is_enabled = $3)`
	QueryInsert  = `INSERT INTO "guliangiam".pricings (id, organization_id, name, display_name, description, plans, is_enabled, trial_duration, application, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, $11, $12)`
	// QueryUpdate — organization_id 取自实体（$10）；调用方必须先做 scoped
	// GetByID，因此跨租户 id 不会到达 UPDATE。0 行表示行已消失/租户不符 → NotFound。
	QueryUpdate = `UPDATE "guliangiam".pricings SET display_name=$1, description=$2, plans=$3::jsonb, is_enabled=$4, trial_duration=$5, application=$6, updated_at=$7, revision=$8 WHERE id=$9 AND organization_id = guliangiam.resolve_org_id($10)`
	QueryDelete = `DELETE FROM "guliangiam".pricings WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
