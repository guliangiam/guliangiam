package ldaps

const btLdaps = "`ldaps`"

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
       COALESCE(filter_fields, '[]') AS filter_fields,
       COALESCE(enable_ssl, false) AS enable_ssl,
       COALESCE(allow_self_signed_cert, false) AS allow_self_signed_cert,
       COALESCE(enable_groups, false) AS enable_groups,
       COALESCE(default_groups, '[]') AS default_groups,
       COALESCE(custom_attributes, '{}') AS custom_attributes,
       is_enabled, sync_interval, last_sync_at, created_at, updated_at, revision
FROM ` + btLdaps + ` WHERE id = ?`

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
       COALESCE(filter_fields, '[]') AS filter_fields,
       COALESCE(enable_ssl, false) AS enable_ssl,
       COALESCE(allow_self_signed_cert, false) AS allow_self_signed_cert,
       COALESCE(enable_groups, false) AS enable_groups,
       COALESCE(default_groups, '[]') AS default_groups,
       COALESCE(custom_attributes, '{}') AS custom_attributes,
       is_enabled, sync_interval, last_sync_at, created_at, updated_at, revision
FROM ` + btLdaps + ` WHERE id = ? AND organization_id = ?`

	// queryListLdaps — orgID×2, keyword×3, is_enabled×2, limit, offset = 9
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
       COALESCE(filter_fields, '[]') AS filter_fields,
       COALESCE(enable_ssl, false) AS enable_ssl,
       COALESCE(allow_self_signed_cert, false) AS allow_self_signed_cert,
       COALESCE(enable_groups, false) AS enable_groups,
       COALESCE(default_groups, '[]') AS default_groups,
       COALESCE(custom_attributes, '{}') AS custom_attributes,
       is_enabled, sync_interval, last_sync_at, created_at, updated_at, revision
FROM ` + btLdaps + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	// queryCountLdaps — orgID×2, keyword×3, is_enabled×2 = 7
	queryCountLdaps = `
SELECT COUNT(*)
FROM ` + btLdaps + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)`

	// queryInsertLdap — 27 个 ?（去掉 ::jsonb）
	queryInsertLdap = `
INSERT INTO ` + btLdaps + ` (id, organization_id, name, display_name, server_name, host, port, admin, bind_dn, bind_password, base_dn, type, attribute_name, password_type, filter, filter_fields, enable_ssl, allow_self_signed_cert, enable_groups, default_groups, custom_attributes, is_enabled, sync_interval, last_sync_at, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdateLdap — SET 21 + updated_at + id + revision + organization_id = 24 个 ?
	// PG 版参数顺序：display_name, server_name, host, port, admin, bind_dn, bind_password, base_dn,
	//   type, attribute_name, password_type, filter, filter_fields, enable_ssl, allow_self_signed_cert,
	//   enable_groups, default_groups, custom_attributes, is_enabled, sync_interval, last_sync_at,
	//   updated_at, revision, id
	// MySQL 版：去掉 RETURNING，revision/organization_id 做 WHERE
	queryUpdateLdap = `
UPDATE ` + btLdaps + `
SET display_name = ?,
    server_name = ?,
    host = ?,
    port = ?,
    admin = ?,
    bind_dn = ?,
    bind_password = ?,
    base_dn = ?,
    type = ?,
    attribute_name = ?,
    password_type = ?,
    filter = ?,
    filter_fields = ?,
    enable_ssl = ?,
    allow_self_signed_cert = ?,
    enable_groups = ?,
    default_groups = ?,
    custom_attributes = ?,
    is_enabled = ?,
    sync_interval = ?,
    last_sync_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

	querySelectRevision = `SELECT revision FROM ` + btLdaps + ` WHERE id = ? AND organization_id = ?`

	queryDeleteLdap = `DELETE FROM ` + btLdaps + ` WHERE id = ? AND organization_id = ?`
)
