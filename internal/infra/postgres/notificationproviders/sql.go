package notificationproviders

const (
	queryGetNotificationProviderByID = `
SELECT id, organization_id, name, provider_type, webhook_url, access_token, config, is_enabled, created_at, updated_at, revision
FROM guliangiam.notification_providers
WHERE id = $1::uuid`

	// queryGetNotificationProviderByIDForOrganization — 占位符：id(1)、organization_id(2)。
	queryGetNotificationProviderByIDForOrganization = `
SELECT id, organization_id, name, provider_type, webhook_url, access_token, config, is_enabled, created_at, updated_at, revision
FROM guliangiam.notification_providers
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListNotificationProviders = `
SELECT id, organization_id, name, provider_type, webhook_url, access_token, config, is_enabled, created_at, updated_at, revision
FROM guliangiam.notification_providers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR provider_type ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountNotificationProviders = `
SELECT COUNT(*)
FROM guliangiam.notification_providers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR provider_type ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)`

	queryInsertNotificationProvider = `
INSERT INTO guliangiam.notification_providers (id, organization_id, provider_id, name, provider_type, webhook_url, access_token, config, is_enabled, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12)`

	queryUpdateNotificationProvider = `
UPDATE guliangiam.notification_providers
SET name = $1,
    provider_type = $2,
    webhook_url = $3,
    access_token = $4,
    config = $5::jsonb,
    is_enabled = $6,
    updated_at = $7,
    revision = revision + 1
WHERE id = $9::uuid AND revision = $8 AND organization_id = guliangiam.resolve_org_id($10) RETURNING revision`

	// queryDeleteNotificationProvider — 占位符：id(1)、organization_id(2)。
	queryDeleteNotificationProvider = `
DELETE FROM guliangiam.notification_providers
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
