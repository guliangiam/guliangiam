package providers

import (
	"fmt"
	"strings"
)

const (
	queryGetProviderByID = `
SELECT id, organization_id, name, display_name, category, provider_type, COALESCE(sub_type, ''), COALESCE(method, ''), client_id, client_secret, COALESCE(client_id2, ''), COALESCE(client_secret2, ''), COALESCE(provider_url, ''), COALESCE(auth_url, ''), COALESCE(token_url, ''), COALESCE(user_info_url, ''), COALESCE(callback_url, ''), COALESCE(signing_alg, ''), COALESCE(scopes, '{}'), COALESCE(idp_entity_id, ''), COALESCE(sso_url, ''), COALESCE(slo_url, ''), COALESCE(metadata, ''), COALESCE(issuer_url, ''), COALESCE(region, ''), COALESCE(host, ''), COALESCE(port, 0), COALESCE(database_type, ''), COALESCE(user_mapping, '{}'::jsonb), COALESCE(template, ''), COALESCE(custom_api, ''), COALESCE(custom_header, ''), COALESCE(custom_body, ''), COALESCE(admin, ''), COALESCE(password, ''), COALESCE(app_id, ''), COALESCE(app_secret, ''), COALESCE(agent_id, ''), COALESCE(content_type, ''), COALESCE(sender, ''), COALESCE(receiver, ''), COALESCE(enabled, true), created_at, updated_at, revision, COALESCE(family, ''), COALESCE(kind, ''), COALESCE(variant, '')
FROM guliangiam.providers
WHERE id = $1::uuid
  AND (CASE WHEN $2 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($2) END)
  AND deleted_at IS NULL`

	// queryGetProviderByIDForOrganization — 管理面 by-ID 读取的唯一入口；
	// 占位符：id(1)、organization_id(2)。跨租户与不存在同为 NotFound。
	queryGetProviderByIDForOrganization = `
SELECT id, organization_id, name, display_name, category, provider_type, COALESCE(sub_type, ''), COALESCE(method, ''), client_id, client_secret, COALESCE(client_id2, ''), COALESCE(client_secret2, ''), COALESCE(provider_url, ''), COALESCE(auth_url, ''), COALESCE(token_url, ''), COALESCE(user_info_url, ''), COALESCE(callback_url, ''), COALESCE(signing_alg, ''), COALESCE(scopes, '{}'), COALESCE(idp_entity_id, ''), COALESCE(sso_url, ''), COALESCE(slo_url, ''), COALESCE(metadata, ''), COALESCE(issuer_url, ''), COALESCE(region, ''), COALESCE(host, ''), COALESCE(port, 0), COALESCE(database_type, ''), COALESCE(user_mapping, '{}'::jsonb), COALESCE(template, ''), COALESCE(custom_api, ''), COALESCE(custom_header, ''), COALESCE(custom_body, ''), COALESCE(admin, ''), COALESCE(password, ''), COALESCE(app_id, ''), COALESCE(app_secret, ''), COALESCE(agent_id, ''), COALESCE(content_type, ''), COALESCE(sender, ''), COALESCE(receiver, ''), COALESCE(enabled, true), created_at, updated_at, revision, COALESCE(family, ''), COALESCE(kind, ''), COALESCE(variant, '')
FROM guliangiam.providers
WHERE id = $1::uuid
  AND organization_id = guliangiam.resolve_org_id($2)
  AND deleted_at IS NULL`

	queryGetProviderConfigByProviderID = `
SELECT pc.provider_id, pc.config_type, pc.config, pc.updated_at
FROM guliangiam.provider_configs pc
WHERE pc.provider_id = $1
  AND pc.config_type <> '_provider_registry_v1'
  AND EXISTS (
    SELECT 1
    FROM guliangiam.providers p
    WHERE p.id = pc.provider_id
      AND p.deleted_at IS NULL
      AND (CASE WHEN $2 = '' THEN TRUE ELSE p.organization_id = guliangiam.resolve_org_id($2) END)
  )
ORDER BY pc.updated_at DESC
LIMIT 1`

	queryGetProviderConfigByType = `
SELECT provider_id, config_type, config, updated_at
FROM guliangiam.provider_configs
WHERE provider_id = $1::uuid AND config_type = $2`

	queryListProviderConfigsByTypePrefix = `
SELECT provider_id, config_type, config, updated_at
FROM guliangiam.provider_configs
WHERE config_type = $1 AND provider_id IN (`

	queryRetirePreviousProviderSecrets = `
UPDATE guliangiam.provider_secrets
SET status = $4, expires_at = $5, updated_at = $6, revision = revision + 1
WHERE provider_id = $1::uuid AND secret_type = $2 AND version < $3
  AND status IN ('active', 'grace')`

	queryInsertProviderSecretStatus = `
INSERT INTO guliangiam.provider_secrets (id, provider_id, secret_type, status, last_rotated_at, expires_at, updated_at, version)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, NULL, $6, $7)`

	queryListProviderSecretStatuses = `
SELECT ps.id,
       ps.provider_id,
       ps.secret_type,
       ps.status,
       COALESCE(ps.last_rotated_at, ps.updated_at) AS last_rotated_at,
       COALESCE(ps.expires_at, ps.updated_at) AS expires_at,
       ps.updated_at,
       ps.version
FROM guliangiam.provider_secrets ps
WHERE ps.provider_id = $1
  AND EXISTS (
    SELECT 1
    FROM guliangiam.providers p
    WHERE p.id = ps.provider_id
      AND p.deleted_at IS NULL
      AND (CASE WHEN $2 = '' THEN TRUE ELSE p.organization_id = guliangiam.resolve_org_id($2) END)
  )
ORDER BY ps.updated_at DESC`

	queryListProviders = `
SELECT id, organization_id, name, display_name, category, provider_type, COALESCE(sub_type, ''), COALESCE(method, ''), client_id, client_secret, COALESCE(client_id2, ''), COALESCE(client_secret2, ''), COALESCE(provider_url, ''), COALESCE(auth_url, ''), COALESCE(token_url, ''), COALESCE(user_info_url, ''), COALESCE(callback_url, ''), COALESCE(signing_alg, ''), COALESCE(scopes, '{}'), COALESCE(idp_entity_id, ''), COALESCE(sso_url, ''), COALESCE(slo_url, ''), COALESCE(metadata, ''), COALESCE(issuer_url, ''), COALESCE(region, ''), COALESCE(host, ''), COALESCE(port, 0), COALESCE(database_type, ''), COALESCE(user_mapping, '{}'::jsonb), COALESCE(template, ''), COALESCE(custom_api, ''), COALESCE(custom_header, ''), COALESCE(custom_body, ''), COALESCE(admin, ''), COALESCE(password, ''), COALESCE(app_id, ''), COALESCE(app_secret, ''), COALESCE(agent_id, ''), COALESCE(content_type, ''), COALESCE(sender, ''), COALESCE(receiver, ''), COALESCE(enabled, true), created_at, updated_at, revision, COALESCE(family, ''), COALESCE(kind, ''), COALESCE(variant, '')
FROM guliangiam.providers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3 = '' OR category = $3)
  AND ($4 = '' OR provider_type = $4)
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $5 OFFSET $6`

	queryCountProviders = `
SELECT COUNT(*)
FROM guliangiam.providers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3 = '' OR category = $3)
  AND ($4 = '' OR provider_type = $4)
  AND deleted_at IS NULL`

	queryInsertProvider = `
INSERT INTO guliangiam.providers (id, organization_id, name, display_name, category, provider_type, sub_type, method, client_id, client_secret, client_id2, client_secret2, provider_url, auth_url, token_url, user_info_url, callback_url, signing_alg, scopes, idp_entity_id, sso_url, slo_url, metadata, issuer_url, region, host, port, database_type, user_mapping, template, custom_api, custom_header, custom_body, admin, password, app_id, app_secret, agent_id, content_type, sender, receiver, enabled, created_at, updated_at, revision, family, kind, variant) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29::jsonb, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45, $46, $47, $48)`

	queryUpdateProvider = `
UPDATE guliangiam.providers
SET display_name = $1,
    category = $2,
    provider_type = $3,
    sub_type = $4,
    method = $5,
    client_id = $6,
    client_secret = $7,
    client_id2 = $8,
    client_secret2 = $9,
    provider_url = $10,
    auth_url = $11,
    token_url = $12,
    user_info_url = $13,
    callback_url = $14,
    signing_alg = $15,
    scopes = $16,
    idp_entity_id = $17,
    sso_url = $18,
    slo_url = $19,
    metadata = $20,
    issuer_url = $21,
    region = $22,
    host = $23,
    port = $24,
    database_type = $25,
    user_mapping = $26::jsonb,
    template = $27,
    custom_api = $28,
    custom_header = $29,
    custom_body = $30,
    admin = $31,
    password = $32,
    app_id = $33,
    app_secret = $34,
    agent_id = $35,
    content_type = $36,
    sender = $37,
    receiver = $38,
    enabled = $39,
    updated_at = $40,
    revision = revision + 1,
    family = $43,
    kind = $44,
    variant = $45,
    name = $46
WHERE id = $42::uuid AND revision = $41
  AND (CASE WHEN $47 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($47) END)
RETURNING revision`

	queryUpsertProviderConfig = `
  INSERT INTO guliangiam.provider_configs (provider_id, config_type, config, updated_at) VALUES ($1::uuid, $2, $3::jsonb, $4)
  ON CONFLICT (provider_id, config_type)
  DO UPDATE SET
    config = EXCLUDED.config,
    updated_at = EXCLUDED.updated_at`

	queryDeleteProvider = `
UPDATE guliangiam.providers SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1::uuid
  AND (CASE WHEN $2 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($2) END)
  AND deleted_at IS NULL`

	queryFindMessagingProviderByChannel = `
SELECT id, organization_id, kind, enabled,
       COALESCE(host, ''), COALESCE(port, 0), COALESCE(admin, ''), COALESCE(password, ''),
       COALESCE(app_id, ''), COALESCE(app_secret, ''), COALESCE(region, ''),
       COALESCE(template, ''), COALESCE(custom_api, ''), COALESCE(custom_header, ''),
       COALESCE(custom_body, ''), COALESCE(content_type, ''), COALESCE(sender, '')
FROM guliangiam.providers
WHERE organization_id = $1::uuid
  AND family = 'identity'
  AND kind = ANY($2::text[])
  AND enabled
  AND deleted_at IS NULL
ORDER BY updated_at DESC
LIMIT 1`

	// queryFindMessagingProviderByApplication joins the application's provider
	// binding to the provider row. The organization predicate is mandatory: an
	// application id is caller-supplied and must never borrow another tenant's
	// provider.
	queryFindMessagingProviderByApplication = `
SELECT p.id, p.organization_id, p.kind, p.enabled,
       COALESCE(p.host, ''), COALESCE(p.port, 0), COALESCE(p.admin, ''), COALESCE(p.password, ''),
       COALESCE(p.app_id, ''), COALESCE(p.app_secret, ''), COALESCE(p.region, ''),
       COALESCE(p.template, ''), COALESCE(p.custom_api, ''), COALESCE(p.custom_header, ''),
       COALESCE(p.custom_body, ''), COALESCE(p.content_type, ''), COALESCE(p.sender, '')
FROM guliangiam.application_provider_bindings b
JOIN guliangiam.providers p ON p.id = b.provider_id
WHERE b.application_id = $1::uuid
  AND p.organization_id = $2::uuid
  AND p.family = 'identity'
  AND p.kind = ANY($3::text[])
  AND p.enabled
  AND p.deleted_at IS NULL
ORDER BY b.created_at DESC
LIMIT 1`
)

func buildListProviderConfigsByTypeQuery(count int) string {
	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d::uuid", i+2)
	}
	return queryListProviderConfigsByTypePrefix + strings.Join(placeholders, ", ") + ") ORDER BY provider_id"
}
