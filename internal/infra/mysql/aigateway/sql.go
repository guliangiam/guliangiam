package aigateway

// SQL 方言转换规则（PG → MySQL）：
//
//	$1, $2, ...                          → ?
//	"guliangiam".table                   → `table`
//	$N::uuid / $N::jsonb / $N::text      → ?
//	guliangiam.resolve_org_id($1)        → CASE WHEN ?='' THEN TRUE ELSE organization_id=? END
//	pgx.Query/QueryRow/Exec              → QueryContext/QueryRowContext/ExecContext
//	JSONB                                → TEXT（JSON 字符串）
//	TIMESTAMPTZ                          → DATETIME(6)
//	FILTER (WHERE col = 'x')             → SUM(CASE WHEN col='x' THEN 1 ELSE 0 END)
//	DATE_TRUNC('day', created_at)::date → DATE(created_at)
//	NOW() - interval 'N days'            → DATE_SUB(NOW(), INTERVAL ? DAY)
//	ON CONFLICT (col) DO UPDATE SET ...  → ON DUPLICATE KEY UPDATE ...
//	RETURNING revision                   → 移除（MySQL 用 RowsAffected 判断冲突）
//	COUNT(*) FILTER (WHERE ...)          → SUM(CASE WHEN ... THEN 1 ELSE 0 END)
//	COALESCE(SUM(col), 0)                → COALESCE(SUM(col), 0)（MySQL 支持）

// ============================================================================
// LLM Providers
// ============================================================================

const (
	queryProviderGetByID = `
SELECT id, organization_id, name, COALESCE(display_name, ''), provider_type, COALESCE(supported_protocols, '[]'), COALESCE(preferred_protocol, ''), COALESCE(base_url, ''), COALESCE(api_key_sealed, ''), COALESCE(api_key_fingerprint, ''), status, COALESCE(request_timeout_ms, 0), COALESCE(max_retries, 0), COALESCE(priority, 0), COALESCE(weight, 0), rpm_limit, tpm_limit, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_providers`" + `
WHERE id = ? AND deleted_at IS NULL`

	// 管理面 by-ID 读取：MySQL 无 RLS，organization_id 谓词是唯一租户闸门。
	queryProviderGetByIDForOrganization = `
SELECT id, organization_id, name, COALESCE(display_name, ''), provider_type, COALESCE(supported_protocols, '[]'), COALESCE(preferred_protocol, ''), COALESCE(base_url, ''), COALESCE(api_key_sealed, ''), COALESCE(api_key_fingerprint, ''), status, COALESCE(request_timeout_ms, 0), COALESCE(max_retries, 0), COALESCE(priority, 0), COALESCE(weight, 0), rpm_limit, tpm_limit, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_providers`" + `
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

	queryProviderGetByName = `
SELECT id, organization_id, name, COALESCE(display_name, ''), provider_type, COALESCE(supported_protocols, '[]'), COALESCE(preferred_protocol, ''), COALESCE(base_url, ''), COALESCE(api_key_sealed, ''), COALESCE(api_key_fingerprint, ''), status, COALESCE(request_timeout_ms, 0), COALESCE(max_retries, 0), COALESCE(priority, 0), COALESCE(weight, 0), rpm_limit, tpm_limit, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_providers`" + `
WHERE organization_id = ? AND name = ? AND deleted_at IS NULL`

	queryProviderList = `
SELECT id, organization_id, name, COALESCE(display_name, ''), provider_type, COALESCE(supported_protocols, '[]'), COALESCE(preferred_protocol, ''), COALESCE(base_url, ''), COALESCE(api_key_sealed, ''), COALESCE(api_key_fingerprint, ''), status, COALESCE(request_timeout_ms, 0), COALESCE(max_retries, 0), COALESCE(priority, 0), COALESCE(weight, 0), rpm_limit, tpm_limit, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_providers`" + `
WHERE organization_id = ? AND deleted_at IS NULL
ORDER BY priority DESC, weight DESC, name ASC`

	queryProviderInsert = `
INSERT INTO ` + "`llm_providers`" + ` (id, organization_id, provider_id, name, display_name, provider_type, supported_protocols, preferred_protocol, base_url, api_key_sealed, api_key_fingerprint, status, request_timeout_ms, max_retries, priority, weight, rpm_limit, tpm_limit, created_at, updated_at, deleted_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// revision 更新通过 WHERE id = ? AND revision = ? 判断，RowsAffected==0 → conflict
	queryProviderUpdate = `
UPDATE ` + "`llm_providers`" + `
SET name = ?,
    display_name = ?,
    provider_type = ?,
    supported_protocols = ?,
    preferred_protocol = ?,
    base_url = ?,
    api_key_sealed = ?,
    api_key_fingerprint = ?,
    status = ?,
    request_timeout_ms = ?,
    max_retries = ?,
    priority = ?,
    weight = ?,
    rpm_limit = ?,
    tpm_limit = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ?`

	queryProviderDelete = `
UPDATE ` + "`llm_providers`" + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND deleted_at IS NULL`

	// 管理面写入：organization_id 由调用方传入的可信租户裁决，而不是实体字段，
	// 避免先读后写窗口里实体被替换成他租户行。
	queryProviderUpdateForOrganization = `
UPDATE ` + "`llm_providers`" + `
SET name = ?,
    display_name = ?,
    provider_type = ?,
    supported_protocols = ?,
    preferred_protocol = ?,
    base_url = ?,
    api_key_sealed = ?,
    api_key_fingerprint = ?,
    status = ?,
    request_timeout_ms = ?,
    max_retries = ?,
    priority = ?,
    weight = ?,
    rpm_limit = ?,
    tpm_limit = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

	queryProviderDeleteForOrganization = `
UPDATE ` + "`llm_providers`" + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`
)

// ============================================================================
// LLM Models
// ============================================================================

const (
	queryModelGetByID = `
SELECT id, organization_id, provider_id, provider_model_name, COALESCE(display_name, ''), COALESCE(capabilities, '{}'), max_context_tokens, max_output_tokens, status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_models`" + `
WHERE id = ? AND deleted_at IS NULL`

	// 管理面 by-ID 读取：MySQL 无 RLS，organization_id 谓词是唯一租户闸门。
	queryModelGetByIDForOrganization = `
SELECT id, organization_id, provider_id, provider_model_name, COALESCE(display_name, ''), COALESCE(capabilities, '{}'), max_context_tokens, max_output_tokens, status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_models`" + `
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

	queryModelList = `
SELECT id, organization_id, provider_id, provider_model_name, COALESCE(display_name, ''), COALESCE(capabilities, '{}'), max_context_tokens, max_output_tokens, status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_models`" + `
WHERE organization_id = ? AND deleted_at IS NULL
ORDER BY provider_id ASC, provider_model_name ASC`

	queryModelListByProvider = `
SELECT id, organization_id, provider_id, provider_model_name, COALESCE(display_name, ''), COALESCE(capabilities, '{}'), max_context_tokens, max_output_tokens, status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_models`" + `
WHERE provider_id = ? AND deleted_at IS NULL
ORDER BY provider_model_name ASC`

	queryModelInsert = `
INSERT INTO ` + "`llm_models`" + ` (id, organization_id, provider_id, provider_model_name, display_name, capabilities, max_context_tokens, max_output_tokens, status, created_at, updated_at, deleted_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// revision 更新：WHERE id = ? AND revision = ?
	queryModelUpdate = `
UPDATE ` + "`llm_models`" + `
SET provider_model_name = ?,
    display_name = ?,
    capabilities = ?,
    max_context_tokens = ?,
    max_output_tokens = ?,
    status = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ?`

	queryModelDelete = `
UPDATE ` + "`llm_models`" + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND deleted_at IS NULL`

	// 管理面写入：organization_id 由调用方传入的可信租户裁决，revision guard 保留。
	queryModelUpdateForOrganization = `
UPDATE ` + "`llm_models`" + `
SET provider_model_name = ?,
    display_name = ?,
    capabilities = ?,
    max_context_tokens = ?,
    max_output_tokens = ?,
    status = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

	queryModelDeleteForOrganization = `
UPDATE ` + "`llm_models`" + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`
)

// ============================================================================
// LLM Model Aliases
// ============================================================================

const (
	queryAliasGetByID = `
SELECT id, organization_id, alias, COALESCE(display_name, ''), status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_model_aliases`" + `
WHERE id = ? AND deleted_at IS NULL`

	// 管理面 by-ID 读取：MySQL 无 RLS，organization_id 谓词是唯一租户闸门。
	queryAliasGetByIDForOrganization = `
SELECT id, organization_id, alias, COALESCE(display_name, ''), status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_model_aliases`" + `
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

	queryAliasGetByName = `
SELECT id, organization_id, alias, COALESCE(display_name, ''), status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_model_aliases`" + `
WHERE organization_id = ? AND alias = ? AND deleted_at IS NULL`

	queryAliasList = `
SELECT id, organization_id, alias, COALESCE(display_name, ''), status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_model_aliases`" + `
WHERE organization_id = ? AND deleted_at IS NULL
ORDER BY alias ASC`

	queryAliasListAll = `
SELECT id, organization_id, alias, COALESCE(display_name, ''), status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_model_aliases`" + `
WHERE deleted_at IS NULL
ORDER BY alias ASC`

	queryAliasInsert = `
INSERT INTO ` + "`llm_model_aliases`" + ` (id, organization_id, alias, display_name, status, created_at, updated_at, deleted_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// revision 更新：WHERE id = ? AND revision = ?
	queryAliasUpdate = `
UPDATE ` + "`llm_model_aliases`" + `
SET alias = ?,
    display_name = ?,
    status = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ?`

	queryAliasDelete = `
UPDATE ` + "`llm_model_aliases`" + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND deleted_at IS NULL`

	// 管理面写入：organization_id 由调用方传入的可信租户裁决，revision guard 保留。
	queryAliasUpdateForOrganization = `
UPDATE ` + "`llm_model_aliases`" + `
SET alias = ?,
    display_name = ?,
    status = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

	queryAliasDeleteForOrganization = `
UPDATE ` + "`llm_model_aliases`" + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`
)

// ============================================================================
// LLM Routes
// ============================================================================

const (
	queryRouteGetByID = `
SELECT id, organization_id, alias_id, model_id, weight, priority, status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_routes`" + `
WHERE id = ? AND deleted_at IS NULL`

	queryRouteListByAlias = `
SELECT id, organization_id, alias_id, model_id, weight, priority, status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_routes`" + `
WHERE alias_id = ? AND deleted_at IS NULL
ORDER BY priority DESC, weight DESC`

	queryRouteList = `
SELECT id, organization_id, alias_id, model_id, weight, priority, status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_routes`" + `
WHERE organization_id = ? AND deleted_at IS NULL
ORDER BY alias_id ASC, priority DESC, weight DESC`

	queryRouteInsert = `
INSERT INTO ` + "`llm_routes`" + ` (id, organization_id, alias_id, model_id, weight, priority, status, created_at, updated_at, deleted_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// revision 更新：WHERE id = ? AND revision = ?
	queryRouteUpdate = `
UPDATE ` + "`llm_routes`" + `
SET model_id = ?,
    weight = ?,
    priority = ?,
    status = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ?`

	queryRouteDelete = `
UPDATE ` + "`llm_routes`" + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND deleted_at IS NULL`
)

// ============================================================================
// LLM API Keys
// ============================================================================

const (
	queryAPIKeyGetByID = `
SELECT id, organization_id, name, key_prefix, key_hash, COALESCE(allowed_aliases, '[]'), rpm_limit, tpm_limit, monthly_budget, COALESCE(currency, ''), status, expires_at, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_api_keys`" + `
WHERE id = ? AND deleted_at IS NULL`

	queryAPIKeyGetByHash = `
SELECT id, organization_id, name, key_prefix, key_hash, COALESCE(allowed_aliases, '[]'), rpm_limit, tpm_limit, monthly_budget, COALESCE(currency, ''), status, expires_at, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_api_keys`" + `
WHERE key_hash = ? AND deleted_at IS NULL`

	queryAPIKeyList = `
SELECT id, organization_id, name, key_prefix, key_hash, COALESCE(allowed_aliases, '[]'), rpm_limit, tpm_limit, monthly_budget, COALESCE(currency, ''), status, expires_at, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_api_keys`" + `
WHERE organization_id = ? AND deleted_at IS NULL
ORDER BY created_at DESC`

	queryAPIKeyInsert = `
INSERT INTO ` + "`llm_api_keys`" + ` (id, organization_id, name, key_prefix, key_hash, allowed_aliases, rpm_limit, tpm_limit, monthly_budget, currency, status, expires_at, created_at, updated_at, deleted_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// revision 更新：WHERE id = ? AND revision = ?
	queryAPIKeyUpdate = `
UPDATE ` + "`llm_api_keys`" + `
SET name = ?,
    key_prefix = ?,
    allowed_aliases = ?,
    rpm_limit = ?,
    tpm_limit = ?,
    monthly_budget = ?,
    currency = ?,
    status = ?,
    expires_at = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ?`

	queryAPIKeyDelete = `
UPDATE ` + "`llm_api_keys`" + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND deleted_at IS NULL`
)

// ============================================================================
// LLM Quota Policies
// ============================================================================

const (
	queryQuotaPolicyGetByID = `
SELECT id, organization_id, scope, scope_ref, limit_type, limit_value, reset_cycle, status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_quota_policies`" + `
WHERE id = ? AND deleted_at IS NULL`

	// 管理面 by-ID 读取：MySQL 无 RLS，organization_id 谓词是唯一租户闸门。
	queryQuotaPolicyGetByIDForOrganization = `
SELECT id, organization_id, scope, scope_ref, limit_type, limit_value, reset_cycle, status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_quota_policies`" + `
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

	queryQuotaPolicyList = `
SELECT id, organization_id, scope, scope_ref, limit_type, limit_value, reset_cycle, status, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_quota_policies`" + `
WHERE organization_id = ? AND deleted_at IS NULL
ORDER BY scope ASC, limit_type ASC`

	queryQuotaPolicyInsert = `
INSERT INTO ` + "`llm_quota_policies`" + ` (id, organization_id, scope, scope_ref, limit_type, limit_value, reset_cycle, status, created_at, updated_at, deleted_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// revision 更新：WHERE id = ? AND revision = ?
	queryQuotaPolicyUpdate = `
UPDATE ` + "`llm_quota_policies`" + `
SET scope = ?,
    scope_ref = ?,
    limit_type = ?,
    limit_value = ?,
    reset_cycle = ?,
    status = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ?`

	queryQuotaPolicyDelete = `
UPDATE ` + "`llm_quota_policies`" + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND deleted_at IS NULL`

	// 管理面写入：organization_id 由调用方传入的可信租户裁决，revision guard 保留。
	queryQuotaPolicyUpdateForOrganization = `
UPDATE ` + "`llm_quota_policies`" + `
SET scope = ?,
    scope_ref = ?,
    limit_type = ?,
    limit_value = ?,
    reset_cycle = ?,
    status = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

	queryQuotaPolicyDeleteForOrganization = `
UPDATE ` + "`llm_quota_policies`" + ` SET deleted_at = NOW(), updated_at = NOW()
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`
)

// ============================================================================
// LLM Price Versions
// ============================================================================

const (
	// GetByModel: 对齐 PG 版 —— 取 effective_from <= NOW() 的最新一条
	queryPriceVersionGetByModel = `
SELECT id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, COALESCE(currency, ''), effective_from, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_price_versions`" + `
WHERE model_id = ? AND deleted_at IS NULL AND effective_from <= NOW()
ORDER BY effective_from DESC
LIMIT 1`

	queryPriceVersionListByModel = `
SELECT id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, COALESCE(currency, ''), effective_from, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_price_versions`" + `
WHERE model_id = ? AND deleted_at IS NULL
ORDER BY effective_from DESC`

	queryPriceVersionInsert = `
INSERT INTO ` + "`llm_price_versions`" + ` (id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, currency, effective_from, created_at, updated_at, deleted_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	queryPriceVersionGetByID = `
SELECT id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, COALESCE(currency, ''), effective_from, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_price_versions`" + `
WHERE id = ? AND deleted_at IS NULL`

	// 管理面 by-ID 读取：MySQL 无 RLS，organization_id 谓词是唯一租户闸门。
	queryPriceVersionGetByIDForOrganization = `
SELECT id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, COALESCE(currency, ''), effective_from, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_price_versions`" + `
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

	queryPriceVersionList = `
SELECT id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, COALESCE(currency, ''), effective_from, created_at, updated_at, deleted_at, revision
FROM ` + "`llm_price_versions`" + `
WHERE organization_id = ? AND deleted_at IS NULL
ORDER BY effective_from DESC, model_id ASC`

	queryPriceVersionUpdate = `
UPDATE ` + "`llm_price_versions`" + `
SET version_no = ?, input_price_per_1m = ?, output_price_per_1m = ?, currency = ?,
    effective_from = ?, updated_at = ?, revision = revision + 1
WHERE id = ? AND deleted_at IS NULL`

	queryPriceVersionSoftDelete = `
UPDATE ` + "`llm_price_versions`" + `
SET deleted_at = ?, updated_at = ?, revision = revision + 1
WHERE id = ? AND deleted_at IS NULL`

	// 管理面写入：organization_id 由调用方传入的可信租户裁决，revision guard 保留。
	queryPriceVersionUpdateForOrganization = `
UPDATE ` + "`llm_price_versions`" + `
SET version_no = ?, input_price_per_1m = ?, output_price_per_1m = ?, currency = ?,
    effective_from = ?, updated_at = ?, revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

	queryPriceVersionDeleteForOrganization = `
UPDATE ` + "`llm_price_versions`" + `
SET deleted_at = ?, updated_at = ?, revision = revision + 1
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`
)

// ============================================================================
// Provider Health（tenant-less：经 provider→organization 归属判定）
// ============================================================================

const (
	queryProviderHealthGetByProvider = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM ` + "`llm_provider_health`" + ` h
WHERE h.provider_id = ?`

	// PG ON CONFLICT (provider_id) DO UPDATE → MySQL ON DUPLICATE KEY UPDATE
	queryProviderHealthUpsert = `
INSERT INTO ` + "`llm_provider_health`" + ` (id, provider_id, state, consecutive_failures, open_until, last_checked_at, last_latency_ms, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    state = VALUES(state),
    consecutive_failures = VALUES(consecutive_failures),
    open_until = VALUES(open_until),
    last_checked_at = VALUES(last_checked_at),
    last_latency_ms = VALUES(last_latency_ms),
    updated_at = VALUES(updated_at)`

	queryProviderHealthListOpen = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM ` + "`llm_provider_health`" + ` h
WHERE h.state = 'open'
ORDER BY h.open_until ASC`

	queryProviderHealthGetByID = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM ` + "`llm_provider_health`" + ` h
WHERE h.id = ?`

	// 与 PG 同构：llm_provider_health 无 organization_id，归属来自
	// provider_id → llm_providers.id；MySQL 无 RLS，此谓词是唯一租户闸门。
	queryProviderHealthGetByIDForOrganization = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM ` + "`llm_provider_health`" + ` h
WHERE h.id = ?
  AND EXISTS (
    SELECT 1 FROM ` + "`llm_providers`" + ` p
    WHERE p.id = h.provider_id AND p.organization_id = ?
  )`

	queryProviderHealthListAll = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM ` + "`llm_provider_health`" + ` h
ORDER BY h.state ASC, h.updated_at DESC`

	queryProviderHealthListByOrganization = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM ` + "`llm_provider_health`" + ` h
WHERE EXISTS (
    SELECT 1 FROM ` + "`llm_providers`" + ` p
    WHERE p.id = h.provider_id AND p.organization_id = ?
  )
ORDER BY h.state ASC, h.updated_at DESC`
)

// ============================================================================
// Usage Logs
// ============================================================================

const (
	queryUsageLogInsert = `
INSERT INTO ` + "`llm_usage_logs`" + ` (id, organization_id, api_key_id, alias_id, provider_id, model_id, price_version_id, protocol_face, attempt_no, input_tokens, output_tokens, total_tokens, cost_amount, currency, status, error_code, ttft_ms, total_latency_ms, upstream_request_id, failover_from, request_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	queryUsageLogList = `
SELECT id, organization_id, api_key_id, alias_id, provider_id, model_id, price_version_id, protocol_face, attempt_no, input_tokens, output_tokens, total_tokens, cost_amount, currency, status, error_code, ttft_ms, total_latency_ms, upstream_request_id, failover_from, request_id, created_at
FROM ` + "`llm_usage_logs`" + `
WHERE organization_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	queryUsageLogCount = `
SELECT COUNT(*) FROM ` + "`llm_usage_logs`" + ` WHERE organization_id = ?`
)

// ============================================================================
// Metering Events（outbox 审计落表）
// ============================================================================

const queryMeteringEventSave = `
INSERT IGNORE INTO ` + "`llm_metering_events`" + `
	(id, outbox_id, organization_id, aggregate_type, aggregate_id, event_type,
	 provider_id, model_id, payload_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

// ============================================================================
// UsageStats — 聚合查询
// ============================================================================

// queryStatsWhereOrg 基础 WHERE 条件（只按 org 过滤）
const queryStatsWhereOrg = "WHERE organization_id = ?"

// queryStatsWhereOrgDays WHERE 条件（按 org + days 过滤）
// MySQL: DATE_SUB(NOW(), INTERVAL ? DAY)
const queryStatsWhereOrgDays = "WHERE organization_id = ? AND created_at >= DATE_SUB(NOW(), INTERVAL ? DAY)"

// queryStatsTotals MySQL 版聚合：
//
//	FILTER (WHERE ...) → SUM(CASE WHEN ... THEN 1 ELSE 0 END)
const queryStatsTotals = `
SELECT
	COUNT(*),
	SUM(CASE WHEN status = 'succeeded' THEN 1 ELSE 0 END),
	SUM(CASE WHEN status IN ('failed','throttled') THEN 1 ELSE 0 END),
	COALESCE(SUM(input_tokens), 0),
	COALESCE(SUM(output_tokens), 0),
	COALESCE(SUM(total_tokens), 0),
	COALESCE(SUM(CASE WHEN currency = 'USD' THEN cost_amount ELSE 0 END), 0),
	AVG(ttft_ms),
	AVG(total_latency_ms)
FROM ` + "`llm_usage_logs`" + ` `

// queryStatsByProvider 按 provider 聚合
const queryStatsByProvider = `
SELECT provider_id,
       COUNT(*),
       COALESCE(SUM(total_tokens), 0),
       COALESCE(SUM(CASE WHEN currency = 'USD' THEN cost_amount ELSE 0 END), 0)
FROM ` + "`llm_usage_logs`" + ` ` + queryStatsWhereOrg + `
GROUP BY provider_id
ORDER BY SUM(total_tokens) DESC
LIMIT 20`

// queryStatsByProviderDays 按 provider 聚合（带 days）
const queryStatsByProviderDays = `
SELECT provider_id,
       COUNT(*),
       COALESCE(SUM(total_tokens), 0),
       COALESCE(SUM(CASE WHEN currency = 'USD' THEN cost_amount ELSE 0 END), 0)
FROM ` + "`llm_usage_logs`" + ` ` + queryStatsWhereOrgDays + `
GROUP BY provider_id
ORDER BY SUM(total_tokens) DESC
LIMIT 20`

// queryStatsByDay 按天聚合：DATE(created_at) 替代 PG DATE_TRUNC(... )::date
const queryStatsByDay = `
SELECT DATE(created_at),
       COALESCE(SUM(total_tokens), 0),
       COALESCE(SUM(CASE WHEN currency = 'USD' THEN cost_amount ELSE 0 END), 0)
FROM ` + "`llm_usage_logs`" + ` ` + queryStatsWhereOrg + `
GROUP BY DATE(created_at)
ORDER BY DATE(created_at) DESC
LIMIT 30`

// queryStatsByDayDays 按天聚合（带 days）
const queryStatsByDayDays = `
SELECT DATE(created_at),
       COALESCE(SUM(total_tokens), 0),
       COALESCE(SUM(CASE WHEN currency = 'USD' THEN cost_amount ELSE 0 END), 0)
FROM ` + "`llm_usage_logs`" + ` ` + queryStatsWhereOrgDays + `
GROUP BY DATE(created_at)
ORDER BY DATE(created_at) DESC
LIMIT 30`

// queryStatsByProtocol 按协议面聚合
const queryStatsByProtocol = `
SELECT protocol_face,
       COUNT(*),
       COALESCE(SUM(total_tokens), 0)
FROM ` + "`llm_usage_logs`" + ` ` + queryStatsWhereOrg + `
GROUP BY protocol_face
ORDER BY COUNT(*) DESC`

// queryStatsByProtocolDays 按协议面聚合（带 days）
const queryStatsByProtocolDays = `
SELECT protocol_face,
       COUNT(*),
       COALESCE(SUM(total_tokens), 0)
FROM ` + "`llm_usage_logs`" + ` ` + queryStatsWhereOrgDays + `
GROUP BY protocol_face
ORDER BY COUNT(*) DESC`
