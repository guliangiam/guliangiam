package ldaps

const (
	queryGetLdapByID = `
SELECT id, organization_id, name, display_name,
       COALESCE(server_name, '') AS server_name,
       host, port,
       COALESCE(admin, '') AS admin,
       bind_dn, bind_password, base_dn,
       COALESCE(type, '') AS type,
       COALESCE(attribute_name, '') AS attribute_name,
       COALESCE(password_type, 'Plain') AS password_type,
       filter,
       COALESCE(filter_fields, '{}') AS filter_fields,
       COALESCE(enable_ssl, false) AS enable_ssl,
       COALESCE(allow_self_signed_cert, false) AS allow_self_signed_cert,
       COALESCE(enable_groups, false) AS enable_groups,
       COALESCE(default_groups, '{}') AS default_groups,
       COALESCE(custom_attributes, '{}'::jsonb) AS custom_attributes,
       is_enabled, sync_interval, last_sync_at, created_at, updated_at, revision
FROM "guliangiam".ldaps
WHERE id = $1::uuid`

	queryGetLdapByIDForOrganization = `
SELECT id, organization_id, name, display_name,
       COALESCE(server_name, '') AS server_name,
       host, port,
       COALESCE(admin, '') AS admin,
       bind_dn, bind_password, base_dn,
       COALESCE(type, '') AS type,
       COALESCE(attribute_name, '') AS attribute_name,
       COALESCE(password_type, 'Plain') AS password_type,
       filter,
       COALESCE(filter_fields, '{}') AS filter_fields,
       COALESCE(enable_ssl, false) AS enable_ssl,
       COALESCE(allow_self_signed_cert, false) AS allow_self_signed_cert,
       COALESCE(enable_groups, false) AS enable_groups,
       COALESCE(default_groups, '{}') AS default_groups,
       COALESCE(custom_attributes, '{}'::jsonb) AS custom_attributes,
       is_enabled, sync_interval, last_sync_at, created_at, updated_at, revision
FROM "guliangiam".ldaps
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListLdaps = `
SELECT id, organization_id, name, display_name,
       COALESCE(server_name, '') AS server_name,
       host, port,
       COALESCE(admin, '') AS admin,
       bind_dn, bind_password, base_dn,
       COALESCE(type, '') AS type,
       COALESCE(attribute_name, '') AS attribute_name,
       COALESCE(password_type, 'Plain') AS password_type,
       filter,
       COALESCE(filter_fields, '{}') AS filter_fields,
       COALESCE(enable_ssl, false) AS enable_ssl,
       COALESCE(allow_self_signed_cert, false) AS allow_self_signed_cert,
       COALESCE(enable_groups, false) AS enable_groups,
       COALESCE(default_groups, '{}') AS default_groups,
       COALESCE(custom_attributes, '{}'::jsonb) AS custom_attributes,
       is_enabled, sync_interval, last_sync_at, created_at, updated_at, revision
FROM "guliangiam".ldaps
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountLdaps = `
SELECT COUNT(*)
FROM "guliangiam".ldaps
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)`

	queryInsertLdap = `
INSERT INTO "guliangiam".ldaps (id, organization_id, name, display_name, server_name, host, port, admin, bind_dn, bind_password, base_dn, type, attribute_name, password_type, filter, filter_fields, enable_ssl, allow_self_signed_cert, enable_groups, default_groups, custom_attributes, is_enabled, sync_interval, last_sync_at, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21::jsonb, $22, $23, $24, $25, $26, $27)`

	queryUpdateLdap = `
UPDATE "guliangiam".ldaps
SET display_name = $1,
    server_name = $2,
    host = $3,
    port = $4,
    admin = $5,
    bind_dn = $6,
    bind_password = $7,
    base_dn = $8,
    type = $9,
    attribute_name = $10,
    password_type = $11,
    filter = $12,
    filter_fields = $13,
    enable_ssl = $14,
    allow_self_signed_cert = $15,
    enable_groups = $16,
    default_groups = $17,
    custom_attributes = $18::jsonb,
    is_enabled = $19,
    sync_interval = $20,
    last_sync_at = $21,
    updated_at = $22,
    revision = revision + 1
WHERE id = $24::uuid AND revision = $23
  AND organization_id = guliangiam.resolve_org_id($25) RETURNING revision`

	queryDeleteLdap = `
DELETE FROM "guliangiam".ldaps
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
