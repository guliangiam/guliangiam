package aigateway

// ===== LLM Providers =====

const (
	queryProviderGetByID = `
SELECT id, organization_id, name, COALESCE(display_name, ''), provider_type, COALESCE(supported_protocols::text, '[]'), COALESCE(preferred_protocol, ''), COALESCE(base_url, ''), COALESCE(api_key_sealed, ''), COALESCE(api_key_fingerprint, ''), status, COALESCE(request_timeout_ms, 0), COALESCE(max_retries, 0), COALESCE(priority, 0), COALESCE(weight, 0), rpm_limit, tpm_limit, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_providers
WHERE id = $1 AND deleted_at IS NULL`

	// 管理面 by-ID 读取：可信租户是显式谓词，跨租户与不存在同为 NotFound。
	queryProviderGetByIDForOrganization = `
SELECT id, organization_id, name, COALESCE(display_name, ''), provider_type, COALESCE(supported_protocols::text, '[]'), COALESCE(preferred_protocol, ''), COALESCE(base_url, ''), COALESCE(api_key_sealed, ''), COALESCE(api_key_fingerprint, ''), status, COALESCE(request_timeout_ms, 0), COALESCE(max_retries, 0), COALESCE(priority, 0), COALESCE(weight, 0), rpm_limit, tpm_limit, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_providers
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`

	queryProviderGetByName = `
SELECT id, organization_id, name, COALESCE(display_name, ''), provider_type, COALESCE(supported_protocols::text, '[]'), COALESCE(preferred_protocol, ''), COALESCE(base_url, ''), COALESCE(api_key_sealed, ''), COALESCE(api_key_fingerprint, ''), status, COALESCE(request_timeout_ms, 0), COALESCE(max_retries, 0), COALESCE(priority, 0), COALESCE(weight, 0), rpm_limit, tpm_limit, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_providers
WHERE organization_id = $1 AND name = $2 AND deleted_at IS NULL`

	queryProviderList = `
SELECT id, organization_id, name, COALESCE(display_name, ''), provider_type, COALESCE(supported_protocols::text, '[]'), COALESCE(preferred_protocol, ''), COALESCE(base_url, ''), COALESCE(api_key_sealed, ''), COALESCE(api_key_fingerprint, ''), status, COALESCE(request_timeout_ms, 0), COALESCE(max_retries, 0), COALESCE(priority, 0), COALESCE(weight, 0), rpm_limit, tpm_limit, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_providers
WHERE organization_id = $1 AND deleted_at IS NULL
ORDER BY priority DESC, weight DESC, name ASC`

	queryProviderInsert = `
INSERT INTO guliangiam.llm_providers (id, organization_id, provider_id, name, display_name, provider_type, supported_protocols, preferred_protocol, base_url, api_key_sealed, api_key_fingerprint, status, request_timeout_ms, max_retries, priority, weight, rpm_limit, tpm_limit, created_at, updated_at, deleted_at, revision)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`

	queryProviderUpdate = `
UPDATE guliangiam.llm_providers
SET name = $1,
    display_name = $2,
    provider_type = $3,
    supported_protocols = $4::jsonb,
    preferred_protocol = $5,
    base_url = $6,
    api_key_sealed = $7,
    api_key_fingerprint = $8,
    status = $9,
    request_timeout_ms = $10,
    max_retries = $11,
    priority = $12,
    weight = $13,
    rpm_limit = $14,
    tpm_limit = $15,
    updated_at = $16,
    deleted_at = $17,
    revision = revision + 1
WHERE id = $19 AND revision = $18 RETURNING revision`

	// 管理面写入：organization_id 由调用方传入的可信租户裁决，而不是实体字段，
	// 避免先读后写窗口里实体被替换成他租户行。
	queryProviderUpdateForOrganization = `
UPDATE guliangiam.llm_providers
SET name = $1,
    display_name = $2,
    provider_type = $3,
    supported_protocols = $4::jsonb,
    preferred_protocol = $5,
    base_url = $6,
    api_key_sealed = $7,
    api_key_fingerprint = $8,
    status = $9,
    request_timeout_ms = $10,
    max_retries = $11,
    priority = $12,
    weight = $13,
    rpm_limit = $14,
    tpm_limit = $15,
    updated_at = $16,
    deleted_at = $17,
    revision = revision + 1
WHERE id = $19 AND revision = $18
  AND organization_id = guliangiam.resolve_org_id($20)
RETURNING revision`

	queryProviderDelete = `
UPDATE guliangiam.llm_providers SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`

	queryProviderDeleteForOrganization = `
UPDATE guliangiam.llm_providers SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`
)

// ===== LLM Models =====

const (
	queryModelGetByID = `
SELECT id, organization_id, provider_id, provider_model_name, COALESCE(display_name, ''), COALESCE(capabilities::text, '{}'), max_context_tokens, max_output_tokens, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_models
WHERE id = $1 AND deleted_at IS NULL`

	// 管理面 by-ID 读取：可信租户是显式谓词，跨租户与不存在同为 NotFound。
	queryModelGetByIDForOrganization = `
SELECT id, organization_id, provider_id, provider_model_name, COALESCE(display_name, ''), COALESCE(capabilities::text, '{}'), max_context_tokens, max_output_tokens, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_models
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`

	queryModelList = `
SELECT id, organization_id, provider_id, provider_model_name, COALESCE(display_name, ''), COALESCE(capabilities::text, '{}'), max_context_tokens, max_output_tokens, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_models
WHERE organization_id = $1 AND deleted_at IS NULL
ORDER BY provider_id ASC, provider_model_name ASC`

	queryModelListByProvider = `
SELECT id, organization_id, provider_id, provider_model_name, COALESCE(display_name, ''), COALESCE(capabilities::text, '{}'), max_context_tokens, max_output_tokens, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_models
WHERE provider_id = $1 AND deleted_at IS NULL
ORDER BY provider_model_name ASC`

	queryModelInsert = `
INSERT INTO guliangiam.llm_models (id, organization_id, provider_id, provider_model_name, display_name, capabilities, max_context_tokens, max_output_tokens, status, created_at, updated_at, deleted_at, revision)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, $11, $12, $13)`

	queryModelUpdate = `
UPDATE guliangiam.llm_models
SET provider_model_name = $1,
    display_name = $2,
    capabilities = $3::jsonb,
    max_context_tokens = $4,
    max_output_tokens = $5,
    status = $6,
    updated_at = $7,
    deleted_at = $8,
    revision = revision + 1
WHERE id = $10 AND revision = $9 RETURNING revision`

	queryModelDelete = `
UPDATE guliangiam.llm_models SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`

	// 管理面写入：organization_id 由调用方传入的可信租户裁决，revision guard 保留。
	queryModelUpdateForOrganization = `
UPDATE guliangiam.llm_models
SET provider_model_name = $1,
    display_name = $2,
    capabilities = $3::jsonb,
    max_context_tokens = $4,
    max_output_tokens = $5,
    status = $6,
    updated_at = $7,
    deleted_at = $8,
    revision = revision + 1
WHERE id = $10 AND revision = $9
  AND organization_id = guliangiam.resolve_org_id($11)
RETURNING revision`

	queryModelDeleteForOrganization = `
UPDATE guliangiam.llm_models SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`
)

// ===== LLM Model Aliases =====

const (
	queryAliasGetByID = `
SELECT id, organization_id, alias, COALESCE(display_name, ''), status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_model_aliases
WHERE id = $1 AND deleted_at IS NULL`

	// 管理面 by-ID 读取：可信租户是显式谓词，跨租户与不存在同为 NotFound。
	queryAliasGetByIDForOrganization = `
SELECT id, organization_id, alias, COALESCE(display_name, ''), status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_model_aliases
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`

	queryAliasGetByName = `
SELECT id, organization_id, alias, COALESCE(display_name, ''), status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_model_aliases
WHERE organization_id = $1::uuid AND alias = $2 AND deleted_at IS NULL`

	queryAliasList = `
SELECT id, organization_id, alias, COALESCE(display_name, ''), status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_model_aliases
WHERE organization_id = $1::uuid AND deleted_at IS NULL
ORDER BY alias ASC`

	queryAliasListAll = `
SELECT id, organization_id, alias, COALESCE(display_name, ''), status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_model_aliases
WHERE deleted_at IS NULL
ORDER BY alias ASC`

	queryAliasInsert = `
INSERT INTO guliangiam.llm_model_aliases (id, organization_id, alias, display_name, status, created_at, updated_at, deleted_at, revision)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	queryAliasUpdate = `
UPDATE guliangiam.llm_model_aliases
SET alias = $1,
    display_name = $2,
    status = $3,
    updated_at = $4,
    deleted_at = $5,
    revision = revision + 1
WHERE id = $7 AND revision = $6 RETURNING revision`

	queryAliasDelete = `
UPDATE guliangiam.llm_model_aliases SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`

	// 管理面写入：organization_id 由调用方传入的可信租户裁决，revision guard 保留。
	queryAliasUpdateForOrganization = `
UPDATE guliangiam.llm_model_aliases
SET alias = $1,
    display_name = $2,
    status = $3,
    updated_at = $4,
    deleted_at = $5,
    revision = revision + 1
WHERE id = $7 AND revision = $6
  AND organization_id = guliangiam.resolve_org_id($8)
RETURNING revision`

	queryAliasDeleteForOrganization = `
UPDATE guliangiam.llm_model_aliases SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`
)

// ===== LLM Routes =====

const (
	queryRouteGetByID = `
SELECT id, organization_id, alias_id, model_id, weight, priority, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_routes
WHERE id = $1 AND deleted_at IS NULL`

	queryRouteListByAlias = `
SELECT id, organization_id, alias_id, model_id, weight, priority, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_routes
WHERE alias_id = $1 AND deleted_at IS NULL
ORDER BY priority DESC, weight DESC`

	queryRouteList = `
SELECT id, organization_id, alias_id, model_id, weight, priority, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_routes
WHERE organization_id = $1 AND deleted_at IS NULL
ORDER BY alias_id ASC, priority DESC, weight DESC`

	queryRouteInsert = `
INSERT INTO guliangiam.llm_routes (id, organization_id, alias_id, model_id, weight, priority, status, created_at, updated_at, deleted_at, revision)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	queryRouteUpdate = `
UPDATE guliangiam.llm_routes
SET model_id = $1,
    weight = $2,
    priority = $3,
    status = $4,
    updated_at = $5,
    deleted_at = $6,
    revision = revision + 1
WHERE id = $8 AND revision = $7 RETURNING revision`

	queryRouteDelete = `
UPDATE guliangiam.llm_routes SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`
)

// ===== LLM API Keys =====

const (
	queryAPIKeyGetByID = `
SELECT id, organization_id, name, key_prefix, key_hash, COALESCE(allowed_aliases::text, '[]'), rpm_limit, tpm_limit, monthly_budget, COALESCE(currency, ''), status, expires_at, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_api_keys
WHERE id = $1 AND deleted_at IS NULL`

	queryAPIKeyGetByHash = `
SELECT id, organization_id, name, key_prefix, key_hash, COALESCE(allowed_aliases::text, '[]'), rpm_limit, tpm_limit, monthly_budget, COALESCE(currency, ''), status, expires_at, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_api_keys
WHERE key_hash = $1 AND deleted_at IS NULL`

	queryAPIKeyList = `
SELECT id, organization_id, name, key_prefix, key_hash, COALESCE(allowed_aliases::text, '[]'), rpm_limit, tpm_limit, monthly_budget, COALESCE(currency, ''), status, expires_at, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_api_keys
WHERE organization_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC`

	queryAPIKeyInsert = `
INSERT INTO guliangiam.llm_api_keys (id, organization_id, name, key_prefix, key_hash, allowed_aliases, rpm_limit, tpm_limit, monthly_budget, currency, status, expires_at, created_at, updated_at, deleted_at, revision)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	queryAPIKeyUpdate = `
UPDATE guliangiam.llm_api_keys
SET name = $1,
    key_prefix = $2,
    allowed_aliases = $3::jsonb,
    rpm_limit = $4,
    tpm_limit = $5,
    monthly_budget = $6,
    currency = $7,
    status = $8,
    expires_at = $9,
    updated_at = $10,
    deleted_at = $11,
    revision = revision + 1
WHERE id = $13 AND revision = $12 RETURNING revision`

	queryAPIKeyDelete = `
UPDATE guliangiam.llm_api_keys SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`
)

// ===== LLM Quota Policies =====

const (
	queryQuotaPolicyGetByID = `
SELECT id, organization_id, scope, scope_ref, limit_type, limit_value, reset_cycle, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_quota_policies
WHERE id = $1 AND deleted_at IS NULL`

	// 管理面 by-ID 读取：可信租户是显式谓词，跨租户与不存在同为 NotFound。
	queryQuotaPolicyGetByIDForOrganization = `
SELECT id, organization_id, scope, scope_ref, limit_type, limit_value, reset_cycle, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_quota_policies
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`

	queryQuotaPolicyList = `
SELECT id, organization_id, scope, scope_ref, limit_type, limit_value, reset_cycle, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_quota_policies
WHERE organization_id = $1 AND deleted_at IS NULL
ORDER BY scope ASC, limit_type ASC`

	// 数据面 enforcement 读取：只取 active 策略（管理面 List 保留全状态）。
	queryQuotaPolicyListActive = `
SELECT id, organization_id, scope, scope_ref, limit_type, limit_value, reset_cycle, status, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_quota_policies
WHERE organization_id = $1 AND status = 'active' AND deleted_at IS NULL
ORDER BY scope ASC, limit_type ASC`

	queryQuotaPolicyInsert = `
INSERT INTO guliangiam.llm_quota_policies (id, organization_id, scope, scope_ref, limit_type, limit_value, reset_cycle, status, created_at, updated_at, deleted_at, revision)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	queryQuotaPolicyUpdate = `
UPDATE guliangiam.llm_quota_policies
SET scope = $1,
    scope_ref = $2,
    limit_type = $3,
    limit_value = $4,
    reset_cycle = $5,
    status = $6,
    updated_at = $7,
    deleted_at = $8,
    revision = revision + 1
WHERE id = $10 AND revision = $9 RETURNING revision`

	queryQuotaPolicyDelete = `
UPDATE guliangiam.llm_quota_policies SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`

	// 管理面写入：organization_id 由调用方传入的可信租户裁决，revision guard 保留。
	queryQuotaPolicyUpdateForOrganization = `
UPDATE guliangiam.llm_quota_policies
SET scope = $1,
    scope_ref = $2,
    limit_type = $3,
    limit_value = $4,
    reset_cycle = $5,
    status = $6,
    updated_at = $7,
    deleted_at = $8,
    revision = revision + 1
WHERE id = $10 AND revision = $9
  AND organization_id = guliangiam.resolve_org_id($11)
RETURNING revision`

	queryQuotaPolicyDeleteForOrganization = `
UPDATE guliangiam.llm_quota_policies SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`
)

// ===== LLM Price Versions =====

const (
	// GetByModel 选择当前生效的价格版本：按 model_id 取 effective_from <= NOW()
	// 的最新一条。对齐设计文档 §15：价格版本可能有未来生效记录（调价预告），
	// 成本计算不能使用尚未生效的版本。
	queryPriceVersionGetByModel = `
SELECT id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, COALESCE(currency, ''), effective_from, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_price_versions
WHERE model_id = $1 AND deleted_at IS NULL AND effective_from <= NOW()
ORDER BY effective_from DESC
LIMIT 1`

	queryPriceVersionListByModel = `
SELECT id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, COALESCE(currency, ''), effective_from, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_price_versions
WHERE model_id = $1 AND deleted_at IS NULL
ORDER BY effective_from DESC`

	queryPriceVersionInsert = `
INSERT INTO guliangiam.llm_price_versions (id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, currency, effective_from, created_at, updated_at, deleted_at, revision)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	queryPriceVersionGetByID = `
SELECT id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, COALESCE(currency, ''), effective_from, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_price_versions
WHERE id = $1 AND deleted_at IS NULL`

	// 管理面 by-ID 读取：可信租户是显式谓词，跨租户与不存在同为 NotFound。
	queryPriceVersionGetByIDForOrganization = `
SELECT id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, COALESCE(currency, ''), effective_from, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_price_versions
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`

	queryPriceVersionList = `
SELECT id, organization_id, model_id, version_no, input_price_per_1m, output_price_per_1m, COALESCE(currency, ''), effective_from, created_at, updated_at, deleted_at, revision
FROM guliangiam.llm_price_versions
WHERE organization_id = $1 AND deleted_at IS NULL
ORDER BY effective_from DESC, model_id ASC`

	queryPriceVersionUpdate = `
UPDATE guliangiam.llm_price_versions
SET version_no = $1, input_price_per_1m = $2, output_price_per_1m = $3, currency = $4,
    effective_from = $5, updated_at = $6, revision = revision + 1
WHERE id = $7 AND deleted_at IS NULL`

	queryPriceVersionSoftDelete = `
UPDATE guliangiam.llm_price_versions
SET deleted_at = $1, updated_at = $2, revision = revision + 1
WHERE id = $3 AND deleted_at IS NULL`

	// 管理面写入：organization_id 由调用方传入的可信租户裁决，revision guard 保留。
	queryPriceVersionUpdateForOrganization = `
UPDATE guliangiam.llm_price_versions
SET version_no = $1, input_price_per_1m = $2, output_price_per_1m = $3, currency = $4,
    effective_from = $5, updated_at = $6, revision = revision + 1
WHERE id = $8 AND revision = $7
  AND organization_id = guliangiam.resolve_org_id($9)
RETURNING revision`

	queryPriceVersionDeleteForOrganization = `
UPDATE guliangiam.llm_price_versions
SET deleted_at = $1, updated_at = $2, revision = revision + 1
WHERE id = $3 AND organization_id = guliangiam.resolve_org_id($4) AND deleted_at IS NULL`
)

// ===== Provider Health（tenant-less：经 provider→organization 归属判定） =====

const (
	queryProviderHealthGetByProvider = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM guliangiam.llm_provider_health h
WHERE h.provider_id = $1`

	queryProviderHealthUpsert = `
INSERT INTO guliangiam.llm_provider_health (id, provider_id, state, consecutive_failures, open_until, last_checked_at, last_latency_ms, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (provider_id)
DO UPDATE SET
    state = EXCLUDED.state,
    consecutive_failures = EXCLUDED.consecutive_failures,
    open_until = EXCLUDED.open_until,
    last_checked_at = EXCLUDED.last_checked_at,
    last_latency_ms = EXCLUDED.last_latency_ms,
    updated_at = EXCLUDED.updated_at`

	queryProviderHealthListOpen = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM guliangiam.llm_provider_health h
WHERE h.state = 'open'
ORDER BY h.open_until ASC`

	queryProviderHealthGetByID = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM guliangiam.llm_provider_health h
WHERE h.id = $1`

	// queryProviderHealthGetByIDForOrganization 把租户判定下推到 SQL：
	// llm_provider_health 无 organization_id，归属来自 provider_id →
	// llm_providers.id 的外键，因此跨租户与不存在得到同一结果（0 行）。
	queryProviderHealthGetByIDForOrganization = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM guliangiam.llm_provider_health h
WHERE h.id = $1
  AND EXISTS (
    SELECT 1 FROM guliangiam.llm_providers p
    WHERE p.id = h.provider_id AND p.organization_id = $2
  )`

	queryProviderHealthListAll = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM guliangiam.llm_provider_health h
ORDER BY h.state ASC, h.updated_at DESC`

	queryProviderHealthListByOrganization = `
SELECT h.id, h.provider_id, h.state, h.consecutive_failures, h.open_until, h.last_checked_at, h.last_latency_ms, h.updated_at
FROM guliangiam.llm_provider_health h
WHERE EXISTS (
    SELECT 1 FROM guliangiam.llm_providers p
    WHERE p.id = h.provider_id AND p.organization_id = $1
  )
ORDER BY h.state ASC, h.updated_at DESC`
)

// ===== Usage Logs =====

const (
	queryUsageLogInsert = `
INSERT INTO guliangiam.llm_usage_logs (id, organization_id, api_key_id, alias_id, provider_id, model_id, price_version_id, protocol_face, attempt_no, input_tokens, output_tokens, total_tokens, cost_amount, currency, status, error_code, ttft_ms, total_latency_ms, upstream_request_id, failover_from, request_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`

	queryUsageLogList = `
SELECT id, organization_id, api_key_id, alias_id, provider_id, model_id, price_version_id, protocol_face, attempt_no, input_tokens, output_tokens, total_tokens, cost_amount, currency, status, error_code, ttft_ms, total_latency_ms, upstream_request_id, failover_from, request_id, created_at
FROM guliangiam.llm_usage_logs
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3`

	queryUsageLogCount = `
SELECT COUNT(*) FROM guliangiam.llm_usage_logs WHERE organization_id = $1`
)
