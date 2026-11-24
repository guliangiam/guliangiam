package permissions

const (
	queryGetPermissionByID = `
SELECT id, organization_id, name, display_name, description, users, groups, roles, domains, resource_type, resources, actions, effect, is_enabled, submitter, approver, status, COALESCE(model, '') AS model, COALESCE(adapter, '') AS adapter, COALESCE(sorter, '') AS sorter, COALESCE(application, '') AS application, approve_time, created_at, updated_at, revision
FROM guliangiam.permissions
WHERE id = $1::uuid`

	queryGetPermissionByIDForOrganization = `
SELECT id, organization_id, name, display_name, description, users, groups, roles, domains, resource_type, resources, actions, effect, is_enabled, submitter, approver, status, COALESCE(model, '') AS model, COALESCE(adapter, '') AS adapter, COALESCE(sorter, '') AS sorter, COALESCE(application, '') AS application, approve_time, created_at, updated_at, revision
FROM guliangiam.permissions
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListPermissions = `
SELECT id, organization_id, name, display_name, description, users, groups, roles, domains, resource_type, resources, actions, effect, is_enabled, submitter, approver, status, COALESCE(model, '') AS model, COALESCE(adapter, '') AS adapter, COALESCE(sorter, '') AS sorter, COALESCE(application, '') AS application, approve_time, created_at, updated_at, revision
FROM guliangiam.permissions
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%' OR description ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
  AND ($4 = '' OR status = $4)
  AND ($5 = '' OR resource_type = $5)
  AND ($6 = '' OR effect = $6)
ORDER BY created_at DESC
LIMIT $7 OFFSET $8`

	queryCountPermissions = `
SELECT COUNT(*)
FROM guliangiam.permissions
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%' OR description ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
  AND ($4 = '' OR status = $4)
  AND ($5 = '' OR resource_type = $5)
  AND ($6 = '' OR effect = $6)`

	queryInsertPermission = `
INSERT INTO guliangiam.permissions (id, organization_id, name, display_name, description, users, groups, roles, domains, resource_type, resources, actions, effect, is_enabled, submitter, approver, status, model, adapter, sorter, application, approve_time, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10, $11::jsonb, $12::jsonb, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)`

	// queryUpdatePermission — 乐观锁 + 租户谓词。organization_id 取自实体（$23），
	// 调用方必须先做 scoped GetByID，因此 0 行语义仍是 revision 冲突。
	queryUpdatePermission = `
UPDATE guliangiam.permissions
SET display_name = $1,
    description = $2,
    users = $3::jsonb,
    groups = $4::jsonb,
    roles = $5::jsonb,
    domains = $6::jsonb,
    resource_type = $7,
    resources = $8::jsonb,
    actions = $9::jsonb,
    effect = $10,
    is_enabled = $11,
    submitter = $12,
    approver = $13,
    status = $14,
    model = $15,
    adapter = $16,
    sorter = $17,
    application = $18,
    approve_time = $19,
    updated_at = $20,
    revision = revision + 1
WHERE id = $22::uuid AND revision = $21 AND organization_id = $23::uuid RETURNING revision`

	queryDeletePermission = `
DELETE FROM guliangiam.permissions
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
