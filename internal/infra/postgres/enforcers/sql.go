package enforcers

const (
	queryGetEnforcerByID = `
SELECT id, organization_id, name, display_name, description, model_id, adapter_id, is_enabled, owner, created_at, updated_at, revision
FROM "guliangiam".enforcers WHERE id = $1::uuid`

	queryGetEnforcerByIDForOrganization = `
SELECT id, organization_id, name, display_name, description, model_id, adapter_id, is_enabled, owner, created_at, updated_at, revision
FROM "guliangiam".enforcers WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListEnforcers = `
SELECT id, organization_id, name, display_name, description, model_id, adapter_id, is_enabled, owner, created_at, updated_at, revision
FROM "guliangiam".enforcers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
ORDER BY created_at DESC LIMIT $4 OFFSET $5`

	queryCountEnforcers = `
SELECT COUNT(*) FROM "guliangiam".enforcers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)`

	queryInsertEnforcer = `
INSERT INTO "guliangiam".enforcers (id, organization_id, name, display_name, description, model_id, adapter_id, is_enabled, owner, created_at, updated_at, revision)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	// queryUpdateEnforcer — 乐观锁 + 租户谓词。organization_id 取自实体（$10），
	// 调用方必须先做 scoped GetByID，因此 0 行语义仍是 revision 冲突。
	queryUpdateEnforcer = `
UPDATE "guliangiam".enforcers SET display_name=$1, description=$2, model_id=$3, adapter_id=$4, is_enabled=$5, owner=$6, updated_at=$7, revision=revision+1 WHERE id=$9 AND revision=$8 AND organization_id=$10::uuid RETURNING revision`

	queryDeleteEnforcer = `DELETE FROM "guliangiam".enforcers WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
