package webhooks

const (
	queryGetWebhookByID = `
SELECT id, organization_id, name, display_name, url, method, headers, events, is_enabled, COALESCE(secret, '') AS secret, retry_count, timeout_ms,
    COALESCE(organization, '') AS organization,
    COALESCE(content_type, 'application/json') AS content_type,
    COALESCE(custom_body, '') AS custom_body,
    COALESCE(custom_headers, '{}'::jsonb) AS custom_headers,
    COALESCE(cert_id, '') AS cert_id,
    COALESCE(is_error, false) AS is_error,
    COALESCE(error_text, '') AS error_text,
    COALESCE(token_fields, '{}') AS token_fields,
    COALESCE(object_fields, '{}') AS object_fields,
    COALESCE(is_user_extended, false) AS is_user_extended,
    COALESCE(single_org_only, false) AS single_org_only,
    COALESCE(max_retries, 3) AS max_retries,
    COALESCE(retry_interval, 60) AS retry_interval,
    COALESCE(use_exponential_backoff, false) AS use_exponential_backoff,
    created_at, updated_at, revision
FROM "guliangiam".webhooks
WHERE id = $1::uuid`

	// queryGetWebhookByIDForOrganization — 占位符：id(1)、organization_id(2)。
	// 管理面 by-ID 读取的唯一入口；跨租户与不存在同为 NotFound。
	queryGetWebhookByIDForOrganization = `
SELECT id, organization_id, name, display_name, url, method, headers, events, is_enabled, COALESCE(secret, '') AS secret, retry_count, timeout_ms,
    COALESCE(organization, '') AS organization,
    COALESCE(content_type, 'application/json') AS content_type,
    COALESCE(custom_body, '') AS custom_body,
    COALESCE(custom_headers, '{}'::jsonb) AS custom_headers,
    COALESCE(cert_id, '') AS cert_id,
    COALESCE(is_error, false) AS is_error,
    COALESCE(error_text, '') AS error_text,
    COALESCE(token_fields, '{}') AS token_fields,
    COALESCE(object_fields, '{}') AS object_fields,
    COALESCE(is_user_extended, false) AS is_user_extended,
    COALESCE(single_org_only, false) AS single_org_only,
    COALESCE(max_retries, 3) AS max_retries,
    COALESCE(retry_interval, 60) AS retry_interval,
    COALESCE(use_exponential_backoff, false) AS use_exponential_backoff,
    created_at, updated_at, revision
FROM "guliangiam".webhooks
WHERE id = $1::uuid
  AND organization_id = guliangiam.resolve_org_id($2)`

	queryListWebhooks = `
SELECT id, organization_id, name, display_name, url, method, headers, events, is_enabled, COALESCE(secret, '') AS secret, retry_count, timeout_ms,
    COALESCE(organization, '') AS organization,
    COALESCE(content_type, 'application/json') AS content_type,
    COALESCE(custom_body, '') AS custom_body,
    COALESCE(custom_headers, '{}'::jsonb) AS custom_headers,
    COALESCE(cert_id, '') AS cert_id,
    COALESCE(is_error, false) AS is_error,
    COALESCE(error_text, '') AS error_text,
    COALESCE(token_fields, '{}') AS token_fields,
    COALESCE(object_fields, '{}') AS object_fields,
    COALESCE(is_user_extended, false) AS is_user_extended,
    COALESCE(single_org_only, false) AS single_org_only,
    COALESCE(max_retries, 3) AS max_retries,
    COALESCE(retry_interval, 60) AS retry_interval,
    COALESCE(use_exponential_backoff, false) AS use_exponential_backoff,
    created_at, updated_at, revision
FROM "guliangiam".webhooks
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
  AND ($4 = '' OR method = $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6`

	queryCountWebhooks = `
SELECT COUNT(*)
FROM "guliangiam".webhooks
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
  AND ($4 = '' OR method = $4)`

	queryInsertWebhook = `
INSERT INTO "guliangiam".webhooks (id, organization_id, name, display_name, url, method, headers, events, is_enabled, secret, retry_count, timeout_ms, organization, content_type, custom_body, custom_headers, cert_id, is_error, error_text, token_fields, object_fields, is_user_extended, single_org_only, max_retries, retry_interval, use_exponential_backoff, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12, $13, $14, $15, $16::jsonb, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)`

	queryUpdateWebhook = `
UPDATE "guliangiam".webhooks
SET display_name = $1,
    url = $2,
    method = $3,
    headers = $4,
    events = $5::jsonb,
    is_enabled = $6,
    secret = $7,
    retry_count = $8,
    timeout_ms = $9,
    organization = $10,
    content_type = $11,
    custom_body = $12,
    custom_headers = $13::jsonb,
    cert_id = $14,
    is_error = $15,
    error_text = $16,
    token_fields = $17,
    object_fields = $18,
    is_user_extended = $19,
    single_org_only = $20,
    max_retries = $21,
    retry_interval = $22,
    use_exponential_backoff = $23,
    updated_at = $24,
    revision = revision + 1
WHERE id = $26::uuid AND revision = $25 RETURNING revision`

	queryDeleteWebhook = `
DELETE FROM "guliangiam".webhooks
WHERE id = $1::uuid`

	queryListWebhooksByOrgAndEvent = `
SELECT id, organization_id, name, display_name, url, method, headers, events, is_enabled, COALESCE(secret, '') AS secret, retry_count, timeout_ms,
    COALESCE(organization, '') AS organization,
    COALESCE(content_type, 'application/json') AS content_type,
    COALESCE(custom_body, '') AS custom_body,
    COALESCE(custom_headers, '{}'::jsonb) AS custom_headers,
    COALESCE(cert_id, '') AS cert_id,
    COALESCE(is_error, false) AS is_error,
    COALESCE(error_text, '') AS error_text,
    COALESCE(token_fields, '{}') AS token_fields,
    COALESCE(object_fields, '{}') AS object_fields,
    COALESCE(is_user_extended, false) AS is_user_extended,
    COALESCE(single_org_only, false) AS single_org_only,
    COALESCE(max_retries, 3) AS max_retries,
    COALESCE(retry_interval, 60) AS retry_interval,
    COALESCE(use_exponential_backoff, false) AS use_exponential_backoff,
    created_at, updated_at, revision
FROM "guliangiam".webhooks
WHERE organization_id = guliangiam.resolve_org_id($1)
  AND is_enabled = true
  AND events @> $2::jsonb`
)
