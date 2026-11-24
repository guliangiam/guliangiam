package groups

const (
	queryGetGroupByID = `
SELECT id, organization_id, name, display_name, manager, contact_email, type, parent_id, is_top_group, is_enabled, COALESCE(homepage, '') AS homepage, COALESCE(users, '[]'::jsonb) AS users, created_at, updated_at, revision
FROM guliangiam.groups
WHERE id = $1::uuid`

	queryGetGroupByIDForOrganization = `
SELECT id, organization_id, name, display_name, manager, contact_email, type, parent_id, is_top_group, is_enabled, COALESCE(homepage, '') AS homepage, COALESCE(users, '[]'::jsonb) AS users, created_at, updated_at, revision
FROM guliangiam.groups
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListGroups = `
SELECT id, organization_id, name, display_name, manager, contact_email, type, parent_id, is_top_group, is_enabled, COALESCE(homepage, '') AS homepage, COALESCE(users, '[]'::jsonb) AS users, created_at, updated_at, revision
FROM guliangiam.groups
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3 = '' OR is_enabled = CASE WHEN $3 = 'active' THEN TRUE WHEN $3 = 'inactive' THEN FALSE ELSE is_enabled END)
  AND ($4 = '' OR type = $4)`

	queryCountGroups = `
SELECT COUNT(*)
FROM guliangiam.groups
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3 = '' OR is_enabled = CASE WHEN $3 = 'active' THEN TRUE WHEN $3 = 'inactive' THEN FALSE ELSE is_enabled END)
  AND ($4 = '' OR type = $4)`

	queryInsertGroup = `
INSERT INTO guliangiam.groups (id, organization_id, name, display_name, manager, contact_email, type, parent_id, is_top_group, is_enabled, homepage, users, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13, $14, $15)`

	// queryUpdateGroup — 乐观锁 + 租户谓词（organization_id 取自实体）。
	queryUpdateGroup = `
UPDATE guliangiam.groups
SET name = $1,
    display_name = $2,
    manager = $3,
    contact_email = $4,
    type = $5,
    parent_id = $6,
    is_top_group = $7,
    is_enabled = $8,
    homepage = $9,
    users = $10::jsonb,
    updated_at = $11,
    revision = revision + 1
WHERE id = $13::uuid AND revision = $12 AND organization_id = $14::uuid RETURNING revision`

	queryDeleteGroup = `
DELETE FROM guliangiam.groups
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryCountChildrenByParentID = `
SELECT COUNT(*)
FROM guliangiam.groups
WHERE organization_id = guliangiam.resolve_org_id($1) AND parent_id = $2::uuid`
)
