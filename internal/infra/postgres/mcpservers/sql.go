package mcpservers

// token 沿用历史列名，但库内保存的是绑定 (table, column, id) 的 v2 信封密文
// （A-6.2）；空串表示“未配置”，仓储在写入前封装、读取时解封。列宽仍是
// VARCHAR(255)，超长信封由仓储拒绝（不回退明文）。
const (
	QueryGetByID = `SELECT id, organization_id, name, display_name, url, token, application, tools, is_enabled, created_at, updated_at, revision FROM "guliangiam".mcp_servers WHERE id = $1::uuid`
	// QueryGetByIDForOrganization binds the by-ID read to the trusted tenant;
	// a foreign row must be indistinguishable from a missing one.
	QueryGetByIDForOrganization = `SELECT id, organization_id, name, display_name, url, token, application, tools, is_enabled, created_at, updated_at, revision FROM "guliangiam".mcp_servers WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList                   = `SELECT id, organization_id, name, display_name, url, token, application, tools, is_enabled, created_at, updated_at, revision FROM "guliangiam".mcp_servers WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::bool IS NULL OR is_enabled = $3) ORDER BY created_at DESC LIMIT $4 OFFSET $5`
	QueryCount                  = `SELECT COUNT(*) FROM "guliangiam".mcp_servers WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND ($3::bool IS NULL OR is_enabled = $3)`
	QueryInsert                 = `INSERT INTO "guliangiam".mcp_servers (id, organization_id, name, display_name, url, token, application, tools, is_enabled, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12)`
	QueryUpdate                 = `UPDATE "guliangiam".mcp_servers SET display_name=$1, url=$2, token=$3, application=$4, tools=$5::jsonb, is_enabled=$6, updated_at=$7, revision=$8 WHERE id=$9::uuid AND organization_id = guliangiam.resolve_org_id($10)`
	QueryDelete                 = `DELETE FROM "guliangiam".mcp_servers WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
