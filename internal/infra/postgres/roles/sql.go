package roles

const (
	queryGetRoleByIDForOrganization = `
SELECT id, organization_id, name, display_name, description, COALESCE(users, '[]'::jsonb) AS users, group_ids, role_ids, domains, is_enabled, created_at, updated_at, revision
FROM guliangiam.roles
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListRoles = `
SELECT id, organization_id, name, display_name, description, COALESCE(users, '[]'::jsonb) AS users, group_ids, role_ids, domains, is_enabled, created_at, updated_at, revision
FROM guliangiam.roles
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%' OR description ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountRoles = `
SELECT COUNT(*)
FROM guliangiam.roles
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%' OR description ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)`

	queryInsertRole = `
INSERT INTO guliangiam.roles (id, organization_id, name, display_name, description, users, group_ids, role_ids, domains, is_enabled, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10, $11, $12, $13)`

	// queryUpdateRole — 乐观锁 + 租户谓词（organization_id 取自实体）。
	queryUpdateRole = `
UPDATE guliangiam.roles
SET display_name = $1,
    description = $2,
    users = $3::jsonb,
    group_ids = $4::jsonb,
    role_ids = $5::jsonb,
    domains = $6::jsonb,
    is_enabled = $7,
    updated_at = $8,
    revision = revision + 1
WHERE id = $10::uuid AND revision = $9 AND organization_id = $11::uuid RETURNING revision`

	queryDeleteRole = `
DELETE FROM guliangiam.roles
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
