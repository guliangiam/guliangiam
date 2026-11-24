package aigateway

import "context"

// Repository interfaces define the persistence contract for each aggregate.
// Infrastructure implementations (postgres/mysql) satisfy these; the Core
// never reaches out to an infra package directly.

type ProvidersRepo interface {
	// GetByID is the system-scoped lookup used by gateway execution and the
	// health probe, which resolve a provider from configuration rather than
	// from a request-scoped id. Management routes must use
	// TenantScopedProvidersRepo.
	GetByID(ctx context.Context, id string) (*LlmProvider, error)
	List(ctx context.Context, organizationID string) ([]*LlmProvider, error)
	Create(ctx context.Context, p *LlmProvider) error
	Update(ctx context.Context, p *LlmProvider) error
	Delete(ctx context.Context, id string) error
	GetByName(ctx context.Context, organizationID, name string) (*LlmProvider, error)
}

// TenantScopedProvidersRepo is the management-plane persistence contract.
// Every method carries the trusted tenant as an explicit predicate, so a
// foreign or missing provider is reported as not found on both PostgreSQL and
// MySQL (the latter has no RLS to fall back on).
type TenantScopedProvidersRepo interface {
	GetByIDForOrganization(ctx context.Context, organizationID, id string) (*LlmProvider, error)
	UpdateForOrganization(ctx context.Context, p *LlmProvider, organizationID string) error
	DeleteForOrganization(ctx context.Context, organizationID, id string) error
}

type ModelsRepo interface {
	// GetByID is the system-scoped lookup used by gateway execution, which
	// resolves a model from route configuration rather than from a
	// request-scoped id. Management routes must use TenantScopedModelsRepo.
	GetByID(ctx context.Context, id string) (*LlmModel, error)
	List(ctx context.Context, organizationID string) ([]*LlmModel, error)
	ListByProvider(ctx context.Context, providerID string) ([]*LlmModel, error)
	Create(ctx context.Context, m *LlmModel) error
	Update(ctx context.Context, m *LlmModel) error
	Delete(ctx context.Context, id string) error
}

// TenantScopedModelsRepo is the management-plane model contract. Every method
// carries the trusted tenant as an explicit predicate, so a foreign or missing
// model is not found on both PostgreSQL and MySQL.
type TenantScopedModelsRepo interface {
	GetByIDForOrganization(ctx context.Context, organizationID, id string) (*LlmModel, error)
	UpdateForOrganization(ctx context.Context, m *LlmModel, organizationID string) error
	DeleteForOrganization(ctx context.Context, organizationID, id string) error
}

type AliasesRepo interface {
	// GetByID is the system-scoped lookup used by gateway execution, which
	// resolves an alias from request configuration rather than from a
	// request-scoped id. Management routes must use TenantScopedAliasesRepo.
	GetByID(ctx context.Context, id string) (*LlmModelAlias, error)
	GetByName(ctx context.Context, organizationID, alias string) (*LlmModelAlias, error)
	List(ctx context.Context, organizationID string) ([]*LlmModelAlias, error)
	ListAll(ctx context.Context) ([]*LlmModelAlias, error) // cross-org, no orgID filter
	Create(ctx context.Context, a *LlmModelAlias) error
	Update(ctx context.Context, a *LlmModelAlias) error
	Delete(ctx context.Context, id string) error
}

// TenantScopedAliasesRepo is the management-plane alias contract. Every method
// carries the trusted tenant as an explicit predicate, so a foreign or missing
// alias is not found on both PostgreSQL and MySQL.
type TenantScopedAliasesRepo interface {
	GetByIDForOrganization(ctx context.Context, organizationID, id string) (*LlmModelAlias, error)
	UpdateForOrganization(ctx context.Context, a *LlmModelAlias, organizationID string) error
	DeleteForOrganization(ctx context.Context, organizationID, id string) error
}

type RoutesRepo interface {
	GetByID(ctx context.Context, id string) (*LlmRoute, error)
	ListByAlias(ctx context.Context, aliasID string) ([]*LlmRoute, error)
	List(ctx context.Context, organizationID string) ([]*LlmRoute, error)
	Create(ctx context.Context, r *LlmRoute) error
	Update(ctx context.Context, r *LlmRoute) error
	Delete(ctx context.Context, id string) error
}

type ApiKeysRepo interface {
	GetByID(ctx context.Context, id string) (*LlmApiKey, error)
	GetByHash(ctx context.Context, hash string) (*LlmApiKey, error)
	List(ctx context.Context, organizationID string) ([]*LlmApiKey, error)
	Create(ctx context.Context, k *LlmApiKey) error
	Update(ctx context.Context, k *LlmApiKey) error
	Delete(ctx context.Context, id string) error
}

type QuotaPoliciesRepo interface {
	GetByID(ctx context.Context, id string) (*LlmQuotaPolicy, error)
	List(ctx context.Context, organizationID string) ([]*LlmQuotaPolicy, error)
	Create(ctx context.Context, q *LlmQuotaPolicy) error
	Update(ctx context.Context, q *LlmQuotaPolicy) error
	Delete(ctx context.Context, id string) error
}

// ActiveQuotaPoliciesLister 是数据面 enforcement 的可选读能力：只返回
// status='active' 的策略。管理面 List 仍返回全部状态（管理 UI 需要看到
// disabled 策略）；数据面通过类型断言优先使用本能力，未实现时回退
// List + 应用层 status 过滤，两条路径语义一致。
type ActiveQuotaPoliciesLister interface {
	ListActive(ctx context.Context, organizationID string) ([]*LlmQuotaPolicy, error)
}

// TenantScopedQuotaPoliciesRepo is the management-plane quota contract. Every
// method carries the trusted tenant as an explicit predicate, so a foreign or
// missing policy is not found on both PostgreSQL and MySQL.
type TenantScopedQuotaPoliciesRepo interface {
	GetByIDForOrganization(ctx context.Context, organizationID, id string) (*LlmQuotaPolicy, error)
	UpdateForOrganization(ctx context.Context, q *LlmQuotaPolicy, organizationID string) error
	DeleteForOrganization(ctx context.Context, organizationID, id string) error
}

type PriceVersionsRepo interface {
	GetByID(ctx context.Context, id string) (*LlmPriceVersion, error)
	GetByModel(ctx context.Context, modelID string) (*LlmPriceVersion, error) // latest by effective_from
	List(ctx context.Context, organizationID string) ([]*LlmPriceVersion, error)
	ListByModel(ctx context.Context, modelID string) ([]*LlmPriceVersion, error)
	Create(ctx context.Context, v *LlmPriceVersion) error
	Update(ctx context.Context, v *LlmPriceVersion) error
	Delete(ctx context.Context, id string) error
}

// TenantScopedPriceVersionsRepo is the management-plane price contract. Every
// method carries the trusted tenant as an explicit predicate, so a foreign or
// missing version is not found on both PostgreSQL and MySQL.
type TenantScopedPriceVersionsRepo interface {
	GetByIDForOrganization(ctx context.Context, organizationID, id string) (*LlmPriceVersion, error)
	UpdateForOrganization(ctx context.Context, v *LlmPriceVersion, organizationID string) error
	DeleteForOrganization(ctx context.Context, organizationID, id string) error
}

// ProviderHealthRepo is special: the table is tenant-less (no organization_id),
// so RLS does not apply and the persistence layer owns tenant scoping. Ownership
// is resolved through llm_provider_health.provider_id → llm_providers.id; the
// *ForOrganization methods carry that predicate in SQL so a cross-tenant record
// is indistinguishable from a missing one.
type ProviderHealthRepo interface {
	// GetByID is the raw, tenant-less lookup. Tenant-facing callers must use
	// GetByIDForOrganization (or resolve the provider from the organization's
	// list) instead.
	GetByID(ctx context.Context, id string) (*ProviderHealth, error)
	GetByIDForOrganization(ctx context.Context, organizationID, id string) (*ProviderHealth, error)
	GetByProvider(ctx context.Context, providerID string) (*ProviderHealth, error)
	ListAll(ctx context.Context) ([]*ProviderHealth, error)
	ListByOrganization(ctx context.Context, organizationID string) ([]*ProviderHealth, error)
	Upsert(ctx context.Context, h *ProviderHealth) error
	// ListOpen returns all ProviderHealth rows whose state is "open", ordered
	// by open_until ascending so the earliest-expired entries come first.
	// Used by the HealthProbeWorker half-open cron.
	ListOpen(ctx context.Context) ([]*ProviderHealth, error)
}

type UsageLogsRepo interface {
	Create(ctx context.Context, u *UsageLog) error
	List(ctx context.Context, organizationID string, limit, offset int) ([]*UsageLog, int64, error)
	// QueryStats returns aggregate statistics for an org's usage logs.
	// days is the lookback window (e.g. 7 = last 7 days). 0 = no filter.
	QueryStats(ctx context.Context, organizationID string, days int) (*UsageStats, error)
}

// UsageStats 是聚合后的用量统计 —— 管理面 Dashboard 用。
// 这些值来自 UsageLogsRepo.QueryStats 的 GROUP BY / SUM / AVG。
type UsageStats struct {
	// Totals (lookback window)
	TotalRequests     int64   `json:"totalRequests"`
	SucceededCount    int64   `json:"succeededCount"`
	FailedCount       int64   `json:"failedCount"`
	TotalInputTokens  int64   `json:"totalInputTokens"`
	TotalOutputTokens int64   `json:"totalOutputTokens"`
	TotalTokens       int64   `json:"totalTokens"`
	TotalCostUSD      float64 `json:"totalCostUsd"`

	// Latency averages (ms)
	AvgTTFTMs    *float64 `json:"avgTtftMs,omitempty"`
	AvgLatencyMs *float64 `json:"avgLatencyMs,omitempty"`

	// ByProvider 按 provider 聚合
	ByProvider []ProviderUsageBreakdown `json:"byProvider"`
	// ByDay 按天聚合（最近 N 天）
	ByDay []DailyUsageBreakdown `json:"byDay"`
	// ByProtocol 按协议面聚合
	ByProtocol []ProtocolUsageBreakdown `json:"byProtocol"`
}

// ProviderUsageBreakdown — 单 provider 的聚合
type ProviderUsageBreakdown struct {
	ProviderID string  `json:"providerId"`
	Requests   int64   `json:"requests"`
	Tokens     int64   `json:"tokens"`
	CostUSD    float64 `json:"costUsd"`
}

// DailyUsageBreakdown — 单天的聚合（YYYY-MM-DD）
type DailyUsageBreakdown struct {
	Date   string  `json:"date"`
	Tokens int64   `json:"tokens"`
	Cost   float64 `json:"costUsd"`
}

// ProtocolUsageBreakdown — 协议面的聚合
type ProtocolUsageBreakdown struct {
	ProtocolFace string `json:"protocolFace"`
	Requests     int64  `json:"requests"`
	Tokens       int64  `json:"tokens"`
}

// MeteringEventRepo persists outbox-delivered audit events for downstream
// billing / analytics consumption. Events come from the Outbox Worker after
// DeliveryHandler.Handle succeeds — a row here means "this outbox event was
// delivered and audited".
//
// 单一 sink 覆盖三种 aggregate_type:
//
//	llm_provider — circuit_open / circuit_close
//	llm_quota    — quota_exceeded
//	llm_usage    — usage_recorded
//
// payload_json 保存原始 outbox payload，方便下游按 event_type 二次解析。
type MeteringEventRepo interface {
	Save(ctx context.Context, e *MeteringEvent) error
}

// MeteringEvent 是 domain-level 的审计事件 —— 不带 pgx / JSON tag，
// 由 infra 层做列映射。
type MeteringEvent struct {
	ID             string  // uuid
	OutboxID       string  // 原始 outbox item uuid
	OrganizationID *string // nullable — 某些事件（如 provider circuit）没有 org
	AggregateType  string  // llm_provider / llm_quota / llm_usage
	AggregateID    *string // nullable — aggregate uuid
	EventType      string  // ai.provider.circuit_opened 等
	ProviderID     *string
	ModelID        *string
	PayloadJSON    []byte // 原始 JSON bytes
}
