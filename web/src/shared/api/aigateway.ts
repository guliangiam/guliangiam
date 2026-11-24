// =============================================================================
// AI Gateway 管理面 API client
// =============================================================================
//
// 对应后端 admin_chi.go 注册的 /api/v1/ai/* 路由。
// 所有类型从 internal/app/aigateway/admin_dto.go 1:1 映射（camelCase JSON）。
// 请求/响应统一通过 requestDocumentJson 发送（带 credentials + CSRF）。
// =============================================================================

import { requestDocumentJson } from "./http";
import type { Paginated } from "./pagination";
import { AI_GATEWAY_BASE } from "@/shared/api/paths";

const BASE = AI_GATEWAY_BASE;

/** 管理面分页参数。省略时与后端一致按 DefaultPageSize=20 服务。 */
export type ListPageParams = {
  page?: number;
  pageSize?: number;
  /** 客户端过滤关键字：AI 网关列表接口返回全量、不支持服务端 keyword，故在前端过滤。 */
  search?: string;
};

/** 与 platform/pagination.DefaultPageSize 保持一致，避免前端出现第二套默认值。 */
export const AI_GATEWAY_DEFAULT_PAGE_SIZE = 20;

/**
 * 客户端关键字过滤：AI 网关列表后端一次性返回全量且无 keyword 参数，
 * 搜索框只能在前端按指定字段做大小写不敏感的包含匹配。
 */
export function matchSearch<T>(item: T, search: string | undefined, fields: (keyof T)[]): boolean {
  const q = search?.trim().toLowerCase();
  if (!q) return true;
  return fields.some((f) =>
    String(item[f] ?? "")
      .toLowerCase()
      .includes(q),
  );
}

function listQuery(params?: ListPageParams): string {
  const qs = new URLSearchParams();
  qs.set("page", String(params?.page ?? 1));
  qs.set("pageSize", String(params?.pageSize ?? AI_GATEWAY_DEFAULT_PAGE_SIZE));
  return qs.toString();
}

/**
 * 供不接受分页控件的调用点（表单下拉、Provider 详情）跨页取回全量。
 * 终止条件使用服务端返回的 total，不复制 MaxPage 边界。
 */
export async function collectAllPages<T>(
  fetchPage: (params: ListPageParams) => Promise<Paginated<T> | undefined>,
): Promise<T[]> {
  const first = await fetchPage({ page: 1 });
  const all = [...(first?.items ?? [])];
  const total = first?.page.total ?? 0;
  let page = 1;
  while (all.length > 0 && all.length < total) {
    page += 1;
    const next = await fetchPage({ page });
    const items = next?.items ?? [];
    if (items.length === 0) {
      break;
    }
    all.push(...items);
  }
  return all;
}

// ── Provider Types ──────────────────────────────────────────────────────────

export interface ProviderCreate {
  name: string;
  displayName?: string;
  providerType: string;
  baseUrl: string;
  apiKey: string;
  status?: string;
  supportedProtocols?: string[];
  preferredProtocol?: string;
  requestTimeoutMs?: number;
  maxRetries?: number;
  priority?: number;
  weight?: number;
  rpmLimit?: number | null;
  tpmLimit?: number | null;
}

export interface ProviderUpdate {
  name?: string | null;
  displayName?: string | null;
  providerType?: string | null;
  baseUrl?: string | null;
  apiKey?: string | null;
  status?: string | null;
  supportedProtocols?: string[];
  preferredProtocol?: string | null;
  requestTimeoutMs?: number | null;
  maxRetries?: number | null;
  priority?: number | null;
  weight?: number | null;
  rpmLimit?: number | null;
  tpmLimit?: number | null;
}

export interface ProviderResponse {
  id: string;
  organizationId: string;
  name: string;
  displayName?: string;
  providerType: string;
  baseUrl: string;
  status: string;
  supportedProtocols?: string[];
  preferredProtocol?: string;
  requestTimeoutMs: number;
  maxRetries: number;
  priority: number;
  weight: number;
  rpmLimit?: number | null;
  tpmLimit?: number | null;
  apiKeyFingerprint: string;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
  revision: number;
}

export interface ProviderTestResult {
  providerId: string;
  reachable: boolean;
  latencyMs?: number;
  statusCode?: number | null;
  error?: string | null;
}

// ── Model Types ─────────────────────────────────────────────────────────────

export interface ModelCreate {
  providerId: string;
  providerModelName: string;
  displayName?: string;
  capabilities?: Record<string, unknown>;
  maxContextTokens?: number | null;
  maxOutputTokens?: number | null;
  status?: string;
}

export interface ModelUpdate {
  providerModelName?: string;
  displayName?: string | null;
  capabilities?: Record<string, unknown>;
  maxContextTokens?: number | null;
  maxOutputTokens?: number | null;
  status?: string | null;
}

export interface ModelResponse {
  id: string;
  organizationId: string;
  providerId: string;
  providerModelName: string;
  displayName?: string;
  capabilities?: Record<string, unknown>;
  maxContextTokens?: number | null;
  maxOutputTokens?: number | null;
  status: string;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
  revision: number;
}

// ── Alias Types ────────────────────────────────────────────────────────────

export interface AliasCreate {
  alias: string;
  displayName?: string;
  status?: string;
}

export interface AliasUpdate {
  alias?: string | null;
  displayName?: string | null;
  status?: string | null;
}

export interface AliasResponse {
  id: string;
  organizationId: string;
  alias: string;
  displayName?: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
  revision: number;
}

// ── Route Types ─────────────────────────────────────────────────────────────

export interface RouteCreate {
  aliasId: string;
  modelId: string;
  priority?: number;
  weight?: number;
  status?: string;
}

export interface RouteUpdate {
  modelId?: string | null;
  priority?: number | null;
  weight?: number | null;
  status?: string | null;
}

export interface RouteResponse {
  id: string;
  organizationId: string;
  aliasId: string;
  aliasName?: string;
  modelId: string;
  modelName?: string;
  providerId?: string;
  providerName?: string;
  priority: number;
  weight: number;
  status: string;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
  revision: number;
}

// ── API Key Types ──────────────────────────────────────────────────────────

export interface APIKeyCreate {
  name: string;
  allowedAliases?: string[];
  rpmLimit?: number | null;
  tpmLimit?: number | null;
  monthlyBudget?: number | null;
  currency?: string;
  expiresAt?: string | null;
}

export interface APIKeyUpdate {
  name?: string | null;
  allowedAliases?: string[];
  rpmLimit?: number | null;
  tpmLimit?: number | null;
  monthlyBudget?: number | null;
  currency?: string | null;
  expiresAt?: string | null;
  status?: string | null;
}

export interface APIKeyResponse {
  id: string;
  organizationId: string;
  name: string;
  keyPrefix: string;
  plaintext?: string; // 仅 Create 响应带一次
  allowedAliases?: string[];
  rpmLimit?: number | null;
  tpmLimit?: number | null;
  monthlyBudget?: number | null;
  currency?: string;
  status: string;
  expiresAt?: string | null;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
  revision: number;
}

// ── Quota Types ─────────────────────────────────────────────────────────────

export interface QuotaCreate {
  scope: string; // organization | api_key | alias
  scopeRef?: string | null;
  limitType: string; // rpm | tpm | cost
  limitValue: number;
  resetCycle: string; // daily | monthly
  status?: string;
}

export interface QuotaUpdate {
  scopeRef?: string | null;
  limitType?: string | null;
  limitValue?: number | null;
  resetCycle?: string | null;
  status?: string | null;
}

export interface QuotaResponse {
  id: string;
  organizationId: string;
  scope: string;
  scopeRef?: string | null;
  limitType: string;
  limitValue: number;
  resetCycle: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
  revision: number;
}

// ── Usage Log Types ─────────────────────────────────────────────────────────

export interface UsageLogResponse {
  id: string;
  organizationId: string;
  apiKeyId?: string | null;
  aliasId: string;
  providerId: string;
  modelId: string;
  protocolFace: string;
  attemptNo: number;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  costAmount: number;
  currency: string;
  status: string;
  errorCode?: string | null;
  ttftMs?: number | null;
  totalLatencyMs?: number | null;
  upstreamRequestId?: string | null;
  failoverFrom?: string | null;
  requestId: string;
  createdAt: string;
}

// =============================================================================
// Provider CRUD
// =============================================================================

export function listProviders(
  params?: ListPageParams,
): Promise<Paginated<ProviderResponse> | undefined> {
  return requestDocumentJson<Paginated<ProviderResponse>>(`${BASE}/providers?${listQuery(params)}`);
}

export function createProvider(body: ProviderCreate): Promise<ProviderResponse | undefined> {
  return requestDocumentJson<ProviderResponse>(`${BASE}/providers`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function getProvider(id: string): Promise<ProviderResponse | undefined> {
  return requestDocumentJson<ProviderResponse>(`${BASE}/providers/${id}`);
}

export function updateProvider(
  id: string,
  body: ProviderUpdate,
): Promise<ProviderResponse | undefined> {
  return requestDocumentJson<ProviderResponse>(`${BASE}/providers/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function deleteProvider(id: string): Promise<unknown> {
  return requestDocumentJson(`${BASE}/providers/${id}`, { method: "DELETE" });
}

export function testProvider(id: string): Promise<ProviderTestResult | undefined> {
  return requestDocumentJson<ProviderTestResult>(`${BASE}/providers/${id}/test`, {
    method: "POST",
  });
}

// =============================================================================
// Model CRUD
// =============================================================================

export function listModels(params?: ListPageParams): Promise<Paginated<ModelResponse> | undefined> {
  return requestDocumentJson<Paginated<ModelResponse>>(`${BASE}/models?${listQuery(params)}`);
}

export function createModel(body: ModelCreate): Promise<ModelResponse | undefined> {
  return requestDocumentJson<ModelResponse>(`${BASE}/models`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function getModel(id: string): Promise<ModelResponse | undefined> {
  return requestDocumentJson<ModelResponse>(`${BASE}/models/${id}`);
}

export function updateModel(id: string, body: ModelUpdate): Promise<ModelResponse | undefined> {
  return requestDocumentJson<ModelResponse>(`${BASE}/models/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function deleteModel(id: string): Promise<unknown> {
  return requestDocumentJson(`${BASE}/models/${id}`, { method: "DELETE" });
}

// =============================================================================
// Alias CRUD
// =============================================================================

export function listAliases(
  params?: ListPageParams,
): Promise<Paginated<AliasResponse> | undefined> {
  return requestDocumentJson<Paginated<AliasResponse>>(`${BASE}/aliases?${listQuery(params)}`);
}

export function createAlias(body: AliasCreate): Promise<AliasResponse | undefined> {
  return requestDocumentJson<AliasResponse>(`${BASE}/aliases`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function getAlias(id: string): Promise<AliasResponse | undefined> {
  return requestDocumentJson<AliasResponse>(`${BASE}/aliases/${id}`);
}

export function updateAlias(id: string, body: AliasUpdate): Promise<AliasResponse | undefined> {
  return requestDocumentJson<AliasResponse>(`${BASE}/aliases/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function deleteAlias(id: string): Promise<unknown> {
  return requestDocumentJson(`${BASE}/aliases/${id}`, { method: "DELETE" });
}

// =============================================================================
// Route CRUD
// =============================================================================

export function listRoutes(params?: ListPageParams): Promise<Paginated<RouteResponse> | undefined> {
  return requestDocumentJson<Paginated<RouteResponse>>(`${BASE}/routes?${listQuery(params)}`);
}

export function createRoute(body: RouteCreate): Promise<RouteResponse | undefined> {
  return requestDocumentJson<RouteResponse>(`${BASE}/routes`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function getRoute(id: string): Promise<RouteResponse | undefined> {
  return requestDocumentJson<RouteResponse>(`${BASE}/routes/${id}`);
}

export function updateRoute(id: string, body: RouteUpdate): Promise<RouteResponse | undefined> {
  return requestDocumentJson<RouteResponse>(`${BASE}/routes/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function deleteRoute(id: string): Promise<unknown> {
  return requestDocumentJson(`${BASE}/routes/${id}`, { method: "DELETE" });
}

// =============================================================================
// API Key CRUD
// =============================================================================

export function listAPIKeys(
  params?: ListPageParams,
): Promise<Paginated<APIKeyResponse> | undefined> {
  return requestDocumentJson<Paginated<APIKeyResponse>>(`${BASE}/api-keys?${listQuery(params)}`);
}

export function createAPIKey(body: APIKeyCreate): Promise<APIKeyResponse | undefined> {
  return requestDocumentJson<APIKeyResponse>(`${BASE}/api-keys`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function getAPIKey(id: string): Promise<APIKeyResponse | undefined> {
  return requestDocumentJson<APIKeyResponse>(`${BASE}/api-keys/${id}`);
}

export function updateAPIKey(id: string, body: APIKeyUpdate): Promise<APIKeyResponse | undefined> {
  return requestDocumentJson<APIKeyResponse>(`${BASE}/api-keys/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function deleteAPIKey(id: string): Promise<unknown> {
  return requestDocumentJson(`${BASE}/api-keys/${id}`, { method: "DELETE" });
}

export function revokeAPIKey(id: string): Promise<unknown> {
  return requestDocumentJson(`${BASE}/api-keys/${id}/revoke`, {
    method: "POST",
  });
}

// =============================================================================
// Quota CRUD
// =============================================================================

export function listQuotas(params?: ListPageParams): Promise<Paginated<QuotaResponse> | undefined> {
  return requestDocumentJson<Paginated<QuotaResponse>>(`${BASE}/quotas?${listQuery(params)}`);
}

export function createQuota(body: QuotaCreate): Promise<QuotaResponse | undefined> {
  return requestDocumentJson<QuotaResponse>(`${BASE}/quotas`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function getQuota(id: string): Promise<QuotaResponse | undefined> {
  return requestDocumentJson<QuotaResponse>(`${BASE}/quotas/${id}`);
}

export function updateQuota(id: string, body: QuotaUpdate): Promise<QuotaResponse | undefined> {
  return requestDocumentJson<QuotaResponse>(`${BASE}/quotas/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function deleteQuota(id: string): Promise<unknown> {
  return requestDocumentJson(`${BASE}/quotas/${id}`, { method: "DELETE" });
}

// =============================================================================
// Usage (read-only)
// =============================================================================

export function listUsageLogs(
  params?: ListPageParams,
): Promise<Paginated<UsageLogResponse> | undefined> {
  return requestDocumentJson<Paginated<UsageLogResponse>>(`${BASE}/usage?${listQuery(params)}`);
}

// =============================================================================
// Price Version Types
// =============================================================================

export interface PriceVersionCreate {
  modelId: string;
  inputPricePer1M: number;
  outputPricePer1M: number;
  currency?: string;
  effectiveFrom: string;
}

export interface PriceVersionUpdate {
  inputPricePer1M?: number | null;
  outputPricePer1M?: number | null;
  currency?: string | null;
  effectiveFrom?: string | null;
}

export interface PriceVersionResponse {
  id: string;
  organizationId: string;
  modelId: string;
  versionNo: number;
  inputPricePer1M: number;
  outputPricePer1M: number;
  currency: string;
  effectiveFrom: string;
  createdAt: string;
  updatedAt: string;
}

// =============================================================================
// Provider Health Type (read-only admin view)
// =============================================================================

export interface ProviderHealthResponse {
  id: string;
  providerId: string;
  state: string;
  consecutiveFailures: number;
  openUntil?: string | null;
  lastCheckedAt?: string | null;
  lastLatencyMs?: number | null;
  updatedAt: string;
}

// =============================================================================
// Price Version CRUD
// =============================================================================

export function listPrices(
  params?: ListPageParams,
): Promise<Paginated<PriceVersionResponse> | undefined> {
  return requestDocumentJson<Paginated<PriceVersionResponse>>(
    `${BASE}/prices?${listQuery(params)}`,
  );
}

export function createPrice(body: PriceVersionCreate): Promise<PriceVersionResponse | undefined> {
  return requestDocumentJson<PriceVersionResponse>(`${BASE}/prices`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function getPrice(id: string): Promise<PriceVersionResponse | undefined> {
  return requestDocumentJson<PriceVersionResponse>(`${BASE}/prices/${id}`);
}

export function updatePrice(
  id: string,
  body: PriceVersionUpdate,
): Promise<PriceVersionResponse | undefined> {
  return requestDocumentJson<PriceVersionResponse>(`${BASE}/prices/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function deletePrice(id: string): Promise<unknown> {
  return requestDocumentJson(`${BASE}/prices/${id}`, { method: "DELETE" });
}

// =============================================================================
// Provider Health (read-only)
// =============================================================================

export function listProviderHealth(
  params?: ListPageParams,
): Promise<Paginated<ProviderHealthResponse> | undefined> {
  return requestDocumentJson<Paginated<ProviderHealthResponse>>(
    `${BASE}/health?${listQuery(params)}`,
  );
}

export function getProviderHealth(id: string): Promise<ProviderHealthResponse | undefined> {
  return requestDocumentJson<ProviderHealthResponse>(`${BASE}/health/${id}`);
}

// =============================================================================
// Usage Stats (Admin Dashboard)
// =============================================================================

export interface ProviderUsageBreakdown {
  providerId: string;
  requests: number;
  tokens: number;
  costUsd: number;
}

export interface DailyUsageBreakdown {
  date: string;
  tokens: number;
  costUsd: number;
}

export interface ProtocolUsageBreakdown {
  protocolFace: string;
  requests: number;
  tokens: number;
}

export interface UsageStats {
  totalRequests: number;
  succeededCount: number;
  failedCount: number;
  totalInputTokens: number;
  totalOutputTokens: number;
  totalTokens: number;
  totalCostUsd: number;
  avgTtftMs?: number | null;
  avgLatencyMs?: number | null;
  byProvider: ProviderUsageBreakdown[];
  byDay: DailyUsageBreakdown[];
  byProtocol: ProtocolUsageBreakdown[];
}

export function fetchUsageStats(days?: number): Promise<UsageStats | undefined> {
  const qs = days && days > 0 ? `?days=${days}` : "";
  return requestDocumentJson<UsageStats>(`${BASE}/usage/stats${qs}`);
}
