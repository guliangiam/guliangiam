package providers

import (
	"strings"

	"github.com/guliangiam/guliangiam/internal/infra/dialect"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// SQL 方言转换规则（PG → MySQL）：
//   $N              → ?
//   guliangiam.t    → `t`（用 btXxx 常量拼接）
//   ILIKE           → LIKE
//   a || b          → CONCAT(a, b)
//   now()           → NOW()
//   ::uuid / ::jsonb / ::text → 删除
//   guliangiam.resolve_org_id(x) → 直接传 organization_id
//   RETURNING       → 删除
//   ON CONFLICT DO UPDATE SET col = EXCLUDED.col → ON DUPLICATE KEY UPDATE col = VALUES(col)

// 表名常量
const (
	btProviders       = "`providers`"
	btProviderConfigs = "`provider_configs`"
	btProviderSecrets = "`provider_secrets`"
)

var (
	// queryGetProviderByID WHERE id = ? AND (? = '' OR organization_id = ?) AND deleted_at IS NULL
	queryGetProviderByID = `
SELECT id, organization_id, name, display_name, category, provider_type, COALESCE(sub_type, ''), COALESCE(method, ''), client_id, client_secret, COALESCE(client_id2, ''), COALESCE(client_secret2, ''), COALESCE(provider_url, ''), COALESCE(auth_url, ''), COALESCE(token_url, ''), COALESCE(user_info_url, ''), COALESCE(callback_url, ''), COALESCE(signing_alg, ''), COALESCE(scopes, '{}'), COALESCE(idp_entity_id, ''), COALESCE(sso_url, ''), COALESCE(slo_url, ''), COALESCE(metadata, ''), COALESCE(issuer_url, ''), COALESCE(region, ''), COALESCE(host, ''), COALESCE(port, 0), COALESCE(database_type, ''), COALESCE(user_mapping, '{}'), COALESCE(template, ''), COALESCE(custom_api, ''), COALESCE(custom_header, ''), COALESCE(custom_body, ''), COALESCE(admin, ''), COALESCE(password, ''), COALESCE(app_id, ''), COALESCE(app_secret, ''), COALESCE(agent_id, ''), COALESCE(content_type, ''), COALESCE(sender, ''), COALESCE(receiver, ''), COALESCE(enabled, true), created_at, updated_at, revision
FROM ` + btProviders + `
WHERE id = ?
  AND (? = '' OR organization_id = ?)
  AND deleted_at IS NULL`

	// queryGetProviderByIDForOrganization — 管理面 by-ID 读取的唯一入口；
	// MySQL 无 RLS，organization_id 谓词是唯一租户边界。
	// 占位符：id(1)、organization_id(2)。
	queryGetProviderByIDForOrganization = `
SELECT id, organization_id, name, display_name, category, provider_type, COALESCE(sub_type, ''), COALESCE(method, ''), client_id, client_secret, COALESCE(client_id2, ''), COALESCE(client_secret2, ''), COALESCE(provider_url, ''), COALESCE(auth_url, ''), COALESCE(token_url, ''), COALESCE(user_info_url, ''), COALESCE(callback_url, ''), COALESCE(signing_alg, ''), COALESCE(scopes, '{}'), COALESCE(idp_entity_id, ''), COALESCE(sso_url, ''), COALESCE(slo_url, ''), COALESCE(metadata, ''), COALESCE(issuer_url, ''), COALESCE(region, ''), COALESCE(host, ''), COALESCE(port, 0), COALESCE(database_type, ''), COALESCE(user_mapping, '{}'), COALESCE(template, ''), COALESCE(custom_api, ''), COALESCE(custom_header, ''), COALESCE(custom_body, ''), COALESCE(admin, ''), COALESCE(password, ''), COALESCE(app_id, ''), COALESCE(app_secret, ''), COALESCE(agent_id, ''), COALESCE(content_type, ''), COALESCE(sender, ''), COALESCE(receiver, ''), COALESCE(enabled, true), created_at, updated_at, revision
FROM ` + btProviders + `
WHERE id = ?
  AND organization_id = ?
  AND deleted_at IS NULL`

	queryGetProviderConfigByProviderID = `
SELECT pc.provider_id, pc.config_type, pc.config, pc.updated_at
FROM ` + btProviderConfigs + ` pc
WHERE pc.provider_id = ?
  AND pc.config_type <> '_provider_registry_v1'
  AND EXISTS (
    SELECT 1
    FROM ` + btProviders + ` p
    WHERE p.id = pc.provider_id
      AND p.deleted_at IS NULL
      AND (? = '' OR p.organization_id = ?)
  )
ORDER BY pc.updated_at DESC
LIMIT 1`

	queryGetProviderConfigByType = `
SELECT provider_id, config_type, config, updated_at
FROM ` + btProviderConfigs + `
WHERE provider_id = ? AND config_type = ?`

	queryListProviderConfigsByTypePrefix = `
SELECT provider_id, config_type, config, updated_at
FROM ` + btProviderConfigs + `
WHERE config_type = ? AND provider_id IN (`

	queryRetirePreviousProviderSecrets = `
UPDATE ` + btProviderSecrets + `
SET status = ?, expires_at = ?, updated_at = ?, revision = revision + 1
WHERE provider_id = ? AND secret_type = ? AND version < ?
  AND status IN ('active', 'grace')`

	queryInsertProviderSecretStatus = `
INSERT INTO ` + btProviderSecrets + ` (id, provider_id, secret_type, status, last_rotated_at, expires_at, updated_at, version)
VALUES (?, ?, ?, ?, ?, NULL, ?, ?)`

	queryListProviderSecretStatuses = `
SELECT ps.id,
       ps.provider_id,
       ps.secret_type,
       ps.status,
       COALESCE(ps.last_rotated_at, ps.updated_at) AS last_rotated_at,
       COALESCE(ps.expires_at, ps.updated_at) AS expires_at,
       ps.updated_at,
       ps.version
FROM ` + btProviderSecrets + ` ps
WHERE ps.provider_id = ?
  AND EXISTS (
    SELECT 1
    FROM ` + btProviders + ` p
    WHERE p.id = ps.provider_id
      AND p.deleted_at IS NULL
      AND (? = '' OR p.organization_id = ?)
  )
ORDER BY ps.updated_at DESC`

	// queryListProviders 参数：orgChk, org, kwChk, kw1, kw2, catChk, cat, ptChk, pt, limit, offset = 11
	queryListProviders = `
SELECT id, organization_id, name, display_name, category, provider_type, COALESCE(sub_type, ''), COALESCE(method, ''), client_id, client_secret, COALESCE(client_id2, ''), COALESCE(client_secret2, ''), COALESCE(provider_url, ''), COALESCE(auth_url, ''), COALESCE(token_url, ''), COALESCE(user_info_url, ''), COALESCE(callback_url, ''), COALESCE(signing_alg, ''), COALESCE(scopes, '{}'), COALESCE(idp_entity_id, ''), COALESCE(sso_url, ''), COALESCE(slo_url, ''), COALESCE(metadata, ''), COALESCE(issuer_url, ''), COALESCE(region, ''), COALESCE(host, ''), COALESCE(port, 0), COALESCE(database_type, ''), COALESCE(user_mapping, '{}'), COALESCE(template, ''), COALESCE(custom_api, ''), COALESCE(custom_header, ''), COALESCE(custom_body, ''), COALESCE(admin, ''), COALESCE(password, ''), COALESCE(app_id, ''), COALESCE(app_secret, ''), COALESCE(agent_id, ''), COALESCE(content_type, ''), COALESCE(sender, ''), COALESCE(receiver, ''), COALESCE(enabled, true), created_at, updated_at, revision
FROM ` + btProviders + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR category = ?)
  AND (? = '' OR provider_type = ?)
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	// queryCountProviders 参数同 List 去掉 limit/offset = 9
	queryCountProviders = `
SELECT COUNT(*)
FROM ` + btProviders + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR category = ?)
  AND (? = '' OR provider_type = ?)
  AND deleted_at IS NULL`

	queryInsertProvider = `
INSERT INTO ` + btProviders + ` (id, organization_id, name, display_name, category, provider_type, sub_type, method, client_id, client_secret, client_id2, client_secret2, provider_url, auth_url, token_url, user_info_url, callback_url, signing_alg, scopes, idp_entity_id, sso_url, slo_url, metadata, issuer_url, region, host, port, database_type, user_mapping, template, custom_api, custom_header, custom_body, admin, password, app_id, app_secret, agent_id, content_type, sender, receiver, enabled, created_at, updated_at, revision, family, kind, variant) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdateProvider 乐观锁更新。
	// PG 版 RETURNING revision → MySQL 版拆成两步：UPDATE + SELECT revision
	// 参数：39 个 SET 值 + updated_at + family/kind/variant + name + WHERE id + revision + org ×2 = 48
	queryUpdateProvider = `
UPDATE ` + btProviders + `
SET display_name = ?,
    category = ?,
    provider_type = ?,
    sub_type = ?,
    method = ?,
    client_id = ?,
    client_secret = ?,
    client_id2 = ?,
    client_secret2 = ?,
    provider_url = ?,
    auth_url = ?,
    token_url = ?,
    user_info_url = ?,
    callback_url = ?,
    signing_alg = ?,
    scopes = ?,
    idp_entity_id = ?,
    sso_url = ?,
    slo_url = ?,
    metadata = ?,
    issuer_url = ?,
    region = ?,
    host = ?,
    port = ?,
    database_type = ?,
    user_mapping = ?,
    template = ?,
    custom_api = ?,
    custom_header = ?,
    custom_body = ?,
    admin = ?,
    password = ?,
    app_id = ?,
    app_secret = ?,
    agent_id = ?,
    content_type = ?,
    sender = ?,
    receiver = ?,
    enabled = ?,
    updated_at = ?,
    revision = revision + 1,
    family = ?,
    kind = ?,
    variant = ?,
    name = ?
WHERE id = ? AND revision = ?
  AND (? = '' OR organization_id = ?)`

	// querySelectRevision 回读 revision
	querySelectRevision = `SELECT revision FROM ` + btProviders + ` WHERE id = ?`

	// queryUpsertProviderConfig — 方言敏感：UPSERT 经 mysqlshared.Dialect.Upsert 生成
	queryUpsertProviderConfig = `INSERT INTO ` + btProviderConfigs + ` (provider_id, config_type, config, updated_at) VALUES (?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{"config", "updated_at"}}, nil) + `
`

	queryDeleteProvider = `
UPDATE ` + btProviders + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ?
  AND (? = '' OR organization_id = ?)
  AND deleted_at IS NULL`
)

func buildListProviderConfigsByTypeQuery(count int) string {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", count), ",")
	return queryListProviderConfigsByTypePrefix + placeholders + ") ORDER BY provider_id"
}
