package keys

const (
	QueryGetByID = `SELECT id, organization_id, name, display_name, key_type, public_key, private_key, algorithm, expires_at, is_enabled, created_at, updated_at, revision FROM "guliangiam".keys WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList    = `SELECT id, organization_id, name, display_name, key_type, public_key, NULL::text AS private_key, algorithm, expires_at, is_enabled, created_at, updated_at, revision FROM "guliangiam".keys WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::text IS NULL OR key_type = $3) AND ($4::boolean IS NULL OR is_enabled = $4) ORDER BY created_at DESC LIMIT $5 OFFSET $6`
	QueryCount   = `SELECT COUNT(*) FROM "guliangiam".keys WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::text IS NULL OR key_type = $3) AND ($4::boolean IS NULL OR is_enabled = $4)`
	QueryInsert  = `INSERT INTO "guliangiam".keys (id, organization_id, name, display_name, key_type, public_key, private_key, algorithm, expires_at, is_enabled, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	QueryUpdate  = `UPDATE "guliangiam".keys SET display_name=$1, public_key=$2, private_key=$3, algorithm=$4, expires_at=$5, is_enabled=$6, updated_at=$7, revision=$8 WHERE id=$9 AND organization_id = guliangiam.resolve_org_id($10)`
	QueryDelete  = `DELETE FROM "guliangiam".keys WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
