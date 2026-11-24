package syncers

const (
	// queryGetSyncerByIDForOrganization — 管理面与运维（run/test-connection）
	// 的唯一 by-ID 读取；占位符：id(1)、organization_id(2)。
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
       COALESCE(column_mapping, '[]'::jsonb) AS column_mapping,
       COALESCE(is_read_only, false) AS is_read_only,
       COALESCE(affinity, '') AS affinity,
       COALESCE(affiliation_table, '') AS affiliation_table,
       COALESCE(avatar_base_url, '') AS avatar_base_url,
       COALESCE(error_text, '') AS error_text,
       last_sync_at, created_at, updated_at, revision
FROM "guliangiam".syncers
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

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
       COALESCE(column_mapping, '[]'::jsonb) AS column_mapping,
       COALESCE(is_read_only, false) AS is_read_only,
       COALESCE(affinity, '') AS affinity,
       COALESCE(affiliation_table, '') AS affiliation_table,
       COALESCE(avatar_base_url, '') AS avatar_base_url,
       COALESCE(error_text, '') AS error_text,
       last_sync_at, created_at, updated_at, revision
FROM "guliangiam".syncers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountSyncers = `
SELECT COUNT(*)
FROM "guliangiam".syncers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)`

	queryInsertSyncer = `
INSERT INTO "guliangiam".syncers (id, organization_id, name, display_name, type, database_type, host, port, database, table_name, username, password, ssl_mode, ssh_type, ssh_host, ssh_port, ssh_username, ssh_password, ssh_cert_id, is_enabled, sync_interval, field_mapping, column_mapping, is_read_only, affinity, affiliation_table, avatar_base_url, error_text, last_sync_at, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23::jsonb, $24, $25, $26, $27, $28, $29, $30, $31, $32)`

	// queryUpdateSyncer — SET 12 + 扩展字段 $14..$27 + revision 守卫 + id + organization_id = $29。
	// 调用方（app 层）已先按租户读取实体并执行 Revision++，因此返回 0 行只可能
	// 是同一租户内的 revision 冲突；跨租户 id 在读取阶段就已 NotFound。
	queryUpdateSyncer = `
UPDATE "guliangiam".syncers
SET display_name = $1,
    host = $2,
    port = $3,
    database = $4,
    table_name = $5,
    username = $6,
    password = $7,
    is_enabled = $8,
    sync_interval = $9,
    field_mapping = $10,
    last_sync_at = $11,
    updated_at = $12,
    revision = revision + 1,
    database_type = $14,
    ssl_mode = $15,
    ssh_type = $16,
    ssh_host = $17,
    ssh_port = $18,
    ssh_username = $19,
    ssh_password = $20,
    ssh_cert_id = $21,
    column_mapping = $22::jsonb,
    is_read_only = $23,
    affinity = $24,
    affiliation_table = $25,
    avatar_base_url = $26,
    error_text = $27
WHERE id = $28::uuid AND revision = $13 AND organization_id = guliangiam.resolve_org_id($29) RETURNING revision`

	// queryDeleteSyncer — 占位符：id(1)、organization_id(2)。
	queryDeleteSyncer = `
DELETE FROM "guliangiam".syncers
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
