package models

const (
	queryGetModelByID = `
SELECT id, organization_id, name, display_name, description, model_text, owner, is_enabled, created_at, updated_at, revision
FROM "guliangiam".models WHERE id = $1::uuid`

	// queryGetModelByIDForOrganization — 占位符：id(1)、organization_id(2)。
	queryGetModelByIDForOrganization = `
SELECT id, organization_id, name, display_name, description, model_text, owner, is_enabled, created_at, updated_at, revision
FROM "guliangiam".models WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListModels = `
SELECT id, organization_id, name, display_name, description, model_text, owner, is_enabled, created_at, updated_at, revision
FROM "guliangiam".models
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
ORDER BY created_at DESC LIMIT $4 OFFSET $5`

	queryCountModels = `
SELECT COUNT(*) FROM "guliangiam".models
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)`

	queryInsertModel = `
INSERT INTO "guliangiam".models (id, organization_id, name, display_name, description, model_text, owner, is_enabled, created_at, updated_at, revision)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	// queryUpdateModel — 乐观锁 + 租户谓词。organization_id 取自实体（$9），
	// 调用方必须先做 scoped GetByID，因此 0 行语义仍是 revision 冲突。
	queryUpdateModel = `
UPDATE "guliangiam".models SET display_name=$1, description=$2, model_text=$3, owner=$4, is_enabled=$5, updated_at=$6, revision=revision+1 WHERE id=$8::uuid AND revision=$7 AND organization_id = guliangiam.resolve_org_id($9) RETURNING revision`

	queryDeleteModel = `DELETE FROM "guliangiam".models WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
