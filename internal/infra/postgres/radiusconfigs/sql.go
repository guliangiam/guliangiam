package radiusconfigs

const (
	// queryGetRadiusConfigByID — 占位符：id(1)、organization_id(2)。
	queryGetRadiusConfigByID = `
SELECT id, organization_id, name, host, port, shared_secret, auth_protocol, is_enabled, created_at, updated_at, revision
FROM guliangiam.radius_configs
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListRadiusConfigs = `
SELECT id, organization_id, name, host, port, shared_secret, auth_protocol, is_enabled, created_at, updated_at, revision
FROM guliangiam.radius_configs
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2::boolean IS NULL OR is_enabled = $2)
  AND ($3 = '' OR auth_protocol = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountRadiusConfigs = `
SELECT COUNT(*)
FROM guliangiam.radius_configs
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2::boolean IS NULL OR is_enabled = $2)
  AND ($3 = '' OR auth_protocol = $3)`

	queryInsertRadiusConfig = `
INSERT INTO guliangiam.radius_configs (id, organization_id, name, host, port, shared_secret, auth_protocol, is_enabled, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	// queryUpdateRadiusConfig — SET 7 + revision 守卫 + id + organization_id = $10。
	// 调用方（app 层）已先按租户读取实体并执行 Revision++，因此返回 0 行只可能
	// 是同一租户内的 revision 冲突；跨租户 id 在读取阶段就已 NotFound。
	queryUpdateRadiusConfig = `
UPDATE guliangiam.radius_configs
SET name = $1,
    host = $2,
    port = $3,
    shared_secret = $4,
    auth_protocol = $5,
    is_enabled = $6,
    updated_at = $7,
    revision = revision + 1
WHERE id = $9::uuid AND revision = $8 AND organization_id = guliangiam.resolve_org_id($10) RETURNING revision`

	// queryDeleteRadiusConfig — 占位符：id(1)、organization_id(2)。
	queryDeleteRadiusConfig = `
DELETE FROM guliangiam.radius_configs
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
