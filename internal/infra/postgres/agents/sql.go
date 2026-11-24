package agents

const (
	queryGetAgentByID = `
SELECT id, organization_id, name, display_name, type, host, port, url, token, application_id, is_enabled, last_ping_at, created_at, updated_at, revision
FROM "guliangiam".agents
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListAgents = `
SELECT id, organization_id, name, display_name, type, host, port, url, token, application_id, is_enabled, last_ping_at, created_at, updated_at, revision
FROM "guliangiam".agents
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountAgents = `
SELECT COUNT(*)
FROM "guliangiam".agents
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)`

	queryInsertAgent = `
INSERT INTO "guliangiam".agents (id, organization_id, name, display_name, type, host, port, url, token, application_id, is_enabled, last_ping_at, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	// queryUpdateAgent — SET 9 + revision 守卫 + id + organization_id = $12。
	// 调用方（app 层）已先按租户读取实体并执行 Revision++，因此返回 0 行只可能
	// 是同一租户内的 revision 冲突；跨租户 id 在读取阶段就已 NotFound。
	queryUpdateAgent = `
UPDATE "guliangiam".agents
SET display_name = $1,
    host = $2,
    port = $3,
    url = $4,
    token = $5,
    application_id = $6,
    is_enabled = $7,
    last_ping_at = $8,
    updated_at = $9,
    revision = revision + 1
WHERE id = $11::uuid AND revision = $10 AND organization_id = guliangiam.resolve_org_id($12) RETURNING revision`

	queryDeleteAgent = `
DELETE FROM "guliangiam".agents
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
