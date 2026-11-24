package forms

const (
	// QueryGetByID — 占位符：id(1)、organization_id(2)。
	QueryGetByID = `SELECT id, organization_id, name, display_name, description, type, properties, is_enabled, created_at, updated_at, revision FROM "guliangiam".forms WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList    = `SELECT id, organization_id, name, display_name, description, type, properties, is_enabled, created_at, updated_at, revision FROM "guliangiam".forms WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::boolean IS NULL OR is_enabled = $3) AND ($4 = '' OR type = $4) ORDER BY created_at DESC LIMIT $5 OFFSET $6`
	QueryCount   = `SELECT COUNT(*) FROM "guliangiam".forms WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::boolean IS NULL OR is_enabled = $3) AND ($4 = '' OR type = $4)`
	QueryInsert  = `INSERT INTO "guliangiam".forms (id, organization_id, name, display_name, description, type, properties, is_enabled, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	// QueryUpdate — SET 7 + id(8) + 实体 organization_id(9)。
	QueryUpdate = `UPDATE "guliangiam".forms SET display_name=$1, description=$2, type=$3, properties=$4, is_enabled=$5, updated_at=$6, revision=$7 WHERE id=$8 AND organization_id = guliangiam.resolve_org_id($9)`
	QueryDelete = `DELETE FROM "guliangiam".forms WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
