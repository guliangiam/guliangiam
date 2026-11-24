package applications

// Auto-split from sql.go — domain: misc

const (
	queryUpdateApplicationCore = `
UPDATE guliangiam.applications
SET slug = $1, display_name = $2, category = $3, app_type = $4,
    description = $5, homepage_url = $6, domain = $7, status = $8,
    client_id = $9, tags = $10::varchar[], logo = $11, is_portal = $12,
    is_visible = $13, is_shared = $14, title = $15, favicon = $16,
    menu_order = $17, menu_mode = $18, updated_at = now(), revision = revision + 1
WHERE id = $19::uuid AND organization_id = guliangiam.resolve_org_id($20)
RETURNING revision`

	// queryDeleteApplication 带租户谓词：跨租户 id 与不存在都返回 0 行。
	// 调用方已先做 scoped 读取，这里作为纵深防御。
	queryDeleteApplication = `
DELETE FROM guliangiam.applications
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryDeleteApplicationProviderBindings = `
DELETE FROM guliangiam.application_provider_bindings WHERE application_id = $1::uuid`

	queryDeleteApplicationConsents = `
DELETE FROM guliangiam.consents WHERE application_id = $1::uuid`

	queryInsertApplicationProviderBinding = `
INSERT INTO guliangiam.application_provider_bindings (application_id, provider_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT DO NOTHING`

	querySetRegistrationAccessToken = `
UPDATE guliangiam.app_oauth_config
SET registration_access_token_hash = $2
WHERE application_id = (SELECT id FROM guliangiam.applications WHERE client_id = $1)`

	queryGetRegistrationAccessTokenHash = `
SELECT oauth.registration_access_token_hash
FROM guliangiam.applications apps
JOIN guliangiam.app_oauth_config oauth ON oauth.application_id = apps.id
WHERE apps.client_id = $1 AND apps.deleted_at IS NULL`
)
