package applications

// Auto-split from sql.go — domain: list

const (
	queryListApplications = `
SELECT apps.id, apps.organization_id, apps.slug, apps.display_name,
       apps.category, apps.app_type, COALESCE(apps.description, ''), COALESCE(apps.homepage_url, ''),
       COALESCE(apps.domain, ''), apps.status, COALESCE(apps.client_id, ''), apps.tags, apps.logo,
       apps.is_portal, apps.is_visible, apps.is_shared, apps.title, apps.favicon,
       apps.menu_order, apps.menu_mode, apps.created_at, apps.updated_at, apps.deleted_at, apps.revision,
       COALESCE(oauth.token_format, '') AS token_format,
       COALESCE(oauth.grant_types, ARRAY[]::text[]) AS grant_types,
       COALESCE(oauth.token_endpoint_auth_method, '') AS token_endpoint_auth_method,
       COALESCE(signin.disable_signin, false) AS disable_signin,
       COALESCE(signin.enable_signup, false) AS enable_signup
FROM guliangiam.applications apps
LEFT JOIN guliangiam.app_oauth_config oauth ON oauth.application_id = apps.id
LEFT JOIN guliangiam.app_signin_config signin ON signin.application_id = apps.id
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE apps.organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR apps.slug ILIKE '%' || $2 || '%' OR apps.display_name ILIKE '%' || $2 || '%' OR apps.domain ILIKE '%' || $2 || '%')
  AND ($3 = '' OR apps.status = $3)
  AND ($4 = '' OR apps.category = $4)
  AND apps.deleted_at IS NULL
ORDER BY apps.created_at DESC
LIMIT $5 OFFSET $6`

	queryCountApplications = `
SELECT COUNT(*)
FROM guliangiam.applications apps
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE apps.organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR apps.slug ILIKE '%' || $2 || '%' OR apps.display_name ILIKE '%' || $2 || '%' OR apps.domain ILIKE '%' || $2 || '%')
  AND ($3 = '' OR apps.status = $3)
  AND ($4 = '' OR apps.category = $4)
  AND apps.deleted_at IS NULL`

	// ====== 写入操作 — 主表 + 子表在 repository.go 事务中分别执行 ======
)
