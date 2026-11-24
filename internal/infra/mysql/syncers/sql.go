package syncers

const btSyncers = "`syncers`"

const (
	// queryGetSyncerByIDForOrganization 是管理面与运维（run/test-connection）的
	// 唯一 by-ID 读取；MySQL 无 RLS，organization_id 谓词是唯一租户边界。
	// 占位符：id(1)、organization_id(2)。
	queryGetSyncerByIDForOrganization = `
SELECT id, organization_id, name, display_name, type,
       COALESCE(database_type, '') AS database_type,
       host, port, database, table_name, username, password,
       COALESCE(ssl_mode, 'disable') AS ssl_mode,
       COALESCE(ssh_type, '') AS ssh_type,
       COALESCE(ssh_host, '') AS ssh_host,
       COALESCE(ssh_port, 0) AS ssh_port,
       COALESCE(ssh_username, '') AS ssh_username,
       COALESCE(ssh_password, '') AS ssh_password,
       COALESCE(ssh_cert_id, '') AS ssh_cert_id,
       is_enabled, sync_interval, field_mapping,
       COALESCE(column_mapping, '[]') AS column_mapping,
       COALESCE(is_read_only, false) AS is_read_only,
       COALESCE(affinity, '') AS affinity,
       COALESCE(affiliation_table, '') AS affiliation_table,
       COALESCE(avatar_base_url, '') AS avatar_base_url,
       COALESCE(error_text, '') AS error_text,
       last_sync_at, created_at, updated_at, revision
FROM ` + btSyncers + `
WHERE id = ? AND organization_id = ?`

	// queryListSyncers — orgID×2, keyword×3, is_enabled×2, limit, offset = 9
	queryListSyncers = `
SELECT id, organization_id, name, display_name, type,
       COALESCE(database_type, '') AS database_type,
       host, port, database, table_name, username, password,
       COALESCE(ssl_mode, 'disable') AS ssl_mode,
       COALESCE(ssh_type, '') AS ssh_type,
       COALESCE(ssh_host, '') AS ssh_host,
       COALESCE(ssh_port, 0) AS ssh_port,
       COALESCE(ssh_username, '') AS ssh_username,
       COALESCE(ssh_password, '') AS ssh_password,
       COALESCE(ssh_cert_id, '') AS ssh_cert_id,
       is_enabled, sync_interval, field_mapping,
       COALESCE(column_mapping, '[]') AS column_mapping,
       COALESCE(is_read_only, false) AS is_read_only,
       COALESCE(affinity, '') AS affinity,
       COALESCE(affiliation_table, '') AS affiliation_table,
       COALESCE(avatar_base_url, '') AS avatar_base_url,
       COALESCE(error_text, '') AS error_text,
       last_sync_at, created_at, updated_at, revision
FROM ` + btSyncers + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	// queryCountSyncers — orgID×2, keyword×3, is_enabled×2 = 7
	queryCountSyncers = `
SELECT COUNT(*)
FROM ` + btSyncers + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)`

	// queryInsertSyncer — 32 个 ?（去掉 ::jsonb）
	queryInsertSyncer = `
INSERT INTO ` + btSyncers + ` (id, organization_id, name, display_name, type, database_type, host, port, database, table_name, username, password, ssl_mode, ssh_type, ssh_host, ssh_port, ssh_username, ssh_password, ssh_cert_id, is_enabled, sync_interval, field_mapping, column_mapping, is_read_only, affinity, affiliation_table, avatar_base_url, error_text, last_sync_at, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdateSyncer — SET 27 + updated_at + id + revision + organization_id = 30 个 ?。
	// app 层已先按租户读取实体并 Revision++，因此 0 affected 只可能是 revision 冲突；
	// 跨租户 id 在读取阶段就已 NotFound。
	queryUpdateSyncer = `
UPDATE ` + btSyncers + `
SET display_name = ?,
    host = ?,
    port = ?,
    database = ?,
    table_name = ?,
    username = ?,
    password = ?,
    is_enabled = ?,
    sync_interval = ?,
    field_mapping = ?,
    last_sync_at = ?,
    updated_at = ?,
    revision = revision + 1,
    database_type = ?,
    ssl_mode = ?,
    ssh_type = ?,
    ssh_host = ?,
    ssh_port = ?,
    ssh_username = ?,
    ssh_password = ?,
    ssh_cert_id = ?,
    column_mapping = ?,
    is_read_only = ?,
    affinity = ?,
    affiliation_table = ?,
    avatar_base_url = ?,
    error_text = ?
WHERE id = ? AND revision = ? AND organization_id = ?`

	// querySelectRevision 回读 revision；organization_id 谓词防止 MySQL（无 RLS）
	// 在 id 复用时读到其他租户的行。
	querySelectRevision = `SELECT revision FROM ` + btSyncers + ` WHERE id = ? AND organization_id = ?`

	// queryDeleteSyncer 占位符：id(1)、organization_id(2)。
	queryDeleteSyncer = `DELETE FROM ` + btSyncers + ` WHERE id = ? AND organization_id = ?`
)
