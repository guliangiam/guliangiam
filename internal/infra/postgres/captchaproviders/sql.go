package captchaproviders

const (
	queryGetCaptchaProviderByID = `
SELECT id, organization_id, name, provider_type, site_key, secret_key, config, is_enabled, created_at, updated_at, revision
FROM guliangiam.captcha_providers
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListCaptchaProviders = `
SELECT id, organization_id, name, provider_type, site_key, secret_key, config, is_enabled, created_at, updated_at, revision
FROM guliangiam.captcha_providers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR provider_type ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountCaptchaProviders = `
SELECT COUNT(*)
FROM guliangiam.captcha_providers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR provider_type ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)`

	queryInsertCaptchaProvider = `
INSERT INTO guliangiam.captcha_providers (id, organization_id, name, provider_type, site_key, secret_key, config, is_enabled, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11)`

	queryUpdateCaptchaProvider = `
UPDATE guliangiam.captcha_providers
SET name = $1,
    provider_type = $2,
    site_key = $3,
    secret_key = $4,
    config = $5::jsonb,
    is_enabled = $6,
    updated_at = $7,
    revision = revision + 1
WHERE id = $9::uuid AND revision = $8 AND organization_id = guliangiam.resolve_org_id($10) RETURNING revision`

	queryDeleteCaptchaProvider = `
DELETE FROM guliangiam.captcha_providers
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
