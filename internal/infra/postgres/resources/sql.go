package resources

const (
	// queryGetResourceByID — 占位符：id(1)、organization_id(2)。跨租户 id 与不存在同为 NotFound。
	queryGetResourceByID = `
SELECT id, organization_id, application_id, name, display_name, type, tag, file_path, file_size, mime_type,
       created_at, updated_at, revision
FROM "guliangiam".resources
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListResources = `
SELECT id, organization_id, application_id, name, display_name, type, tag, file_path, file_size, mime_type,
       created_at, updated_at, revision
FROM "guliangiam".resources
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND (CASE WHEN $2 = '' THEN TRUE ELSE application_id = $2::uuid END)
  AND ($3 = '' OR name ILIKE '%' || $3 || '%' OR display_name ILIKE '%' || $3 || '%')
  AND ($4 = '' OR type = $4)
  AND ($5 = '' OR tag = $5)
ORDER BY created_at DESC
LIMIT $6 OFFSET $7`

	queryCountResources = `
SELECT COUNT(*)
FROM "guliangiam".resources
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND (CASE WHEN $2 = '' THEN TRUE ELSE application_id = $2::uuid END)
  AND ($3 = '' OR name ILIKE '%' || $3 || '%' OR display_name ILIKE '%' || $3 || '%')
  AND ($4 = '' OR type = $4)
  AND ($5 = '' OR tag = $5)`

	queryInsertResource = `
INSERT INTO "guliangiam".resources
  (id, organization_id, application_id, name, display_name, type, tag, file_path, file_size, mime_type, created_at, updated_at, revision)
VALUES ($1::uuid,
        guliangiam.resolve_org_id($2),
        CASE WHEN $3 = '' THEN NULL
             ELSE COALESCE(
                 (SELECT id FROM guliangiam.applications WHERE id::text = $3 OR slug = $3 LIMIT 1),
                 $3::uuid
             )
        END,
        $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	// queryUpdateResource — 乐观锁 + 租户谓词。organization_id 取自实体（$10）；
	// 调用方必须先做 scoped GetByID，因此 0 行语义仍是 revision 冲突。
	queryUpdateResource = `
UPDATE "guliangiam".resources
SET display_name = $1,
    type = $2,
    tag = $3,
    file_path = $4,
    file_size = $5,
    mime_type = $6,
    updated_at = $7,
    revision = revision + 1
WHERE id = $9::uuid AND revision = $8 AND organization_id = guliangiam.resolve_org_id($10) RETURNING revision`

	// queryDeleteResource — 占位符：id(1)、organization_id(2)。
	queryDeleteResource = `
DELETE FROM "guliangiam".resources
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
