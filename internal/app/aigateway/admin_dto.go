package aigateway

// ============================================================================
// Provider DTOs
// ============================================================================

// ProviderCreate 是管理面创建 Provider 的请求体。
// OrganizationID 在 transport 层从 session 注入，JSON 体不携带。
// APIKey 为明文，service 层加密后落库。
type ProviderCreate struct {
	OrganizationID     string   `json:"-"`
	Name               string   `json:"name"`
	DisplayName        string   `json:"displayName,omitempty"`
	ProviderType       string   `json:"providerType"`
	BaseURL            string   `json:"baseUrl"`
	APIKey             string   `json:"apiKey"`
	Status             string   `json:"status,omitempty"`
	SupportedProtocols []string `json:"supportedProtocols,omitempty"`
	PreferredProtocol  string   `json:"preferredProtocol,omitempty"`
	RequestTimeoutMs   int      `json:"requestTimeoutMs,omitempty"`
	MaxRetries         int      `json:"maxRetries,omitempty"`
	Priority           int      `json:"priority,omitempty"`
	Weight             int      `json:"weight,omitempty"`
	RPMLimit           *int     `json:"rpmLimit,omitempty"`
	TPMLimit           *int     `json:"tpmLimit,omitempty"`
}

// ProviderUpdate 是管理面更新 Provider 的请求体。所有字段可选，
// 只有显式发送的字段会被覆盖；APIKey 只有在 key 轮换时才应发送。
type ProviderUpdate struct {
	Name               *string  `json:"name,omitempty"`
	DisplayName        *string  `json:"displayName,omitempty"`
	ProviderType       *string  `json:"providerType,omitempty"`
	BaseURL            *string  `json:"baseUrl,omitempty"`
	APIKey             *string  `json:"apiKey,omitempty"`
	Status             *string  `json:"status,omitempty"`
	SupportedProtocols []string `json:"supportedProtocols,omitempty"`
	PreferredProtocol  *string  `json:"preferredProtocol,omitempty"`
	RequestTimeoutMs   *int     `json:"requestTimeoutMs,omitempty"`
	MaxRetries         *int     `json:"maxRetries,omitempty"`
	Priority           *int     `json:"priority,omitempty"`
	Weight             *int     `json:"weight,omitempty"`
	RPMLimit           *int     `json:"rpmLimit,omitempty"`
	TPMLimit           *int     `json:"tpmLimit,omitempty"`
}

// ProviderResponse 是管理面 Provider 的返回体，敏感字段（APIKeySealed）不暴露。
type ProviderResponse struct {
	ID                 string   `json:"id"`
	OrganizationID     string   `json:"organizationId"`
	Name               string   `json:"name"`
	DisplayName        string   `json:"displayName,omitempty"`
	ProviderType       string   `json:"providerType"`
	BaseURL            string   `json:"baseUrl"`
	Status             string   `json:"status"`
	SupportedProtocols []string `json:"supportedProtocols,omitempty"`
	PreferredProtocol  string   `json:"preferredProtocol,omitempty"`
	RequestTimeoutMs   int      `json:"requestTimeoutMs"`
	MaxRetries         int      `json:"maxRetries"`
	Priority           int      `json:"priority"`
	Weight             int      `json:"weight"`
	RPMLimit           *int     `json:"rpmLimit,omitempty"`
	TPMLimit           *int     `json:"tpmLimit,omitempty"`
	APIKeyFingerprint  string   `json:"apiKeyFingerprint"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
}

// ============================================================================
// Model DTOs
// ============================================================================

// ModelCreate 创建 LlmModel。
type ModelCreate struct {
	OrganizationID    string         `json:"-"`
	ProviderID        string         `json:"providerId"`
	ProviderModelName string         `json:"providerModelName"`
	DisplayName       string         `json:"displayName,omitempty"`
	Capabilities      map[string]any `json:"capabilities,omitempty"`
	MaxContextTokens  *int           `json:"maxContextTokens,omitempty"`
	MaxOutputTokens   *int           `json:"maxOutputTokens,omitempty"`
	Status            string         `json:"status,omitempty"`
}

// ModelUpdate 更新 LlmModel。
type ModelUpdate struct {
	ProviderModelName string         `json:"providerModelName,omitempty"`
	DisplayName       *string        `json:"displayName,omitempty"`
	Capabilities      map[string]any `json:"capabilities,omitempty"`
	MaxContextTokens  *int           `json:"maxContextTokens,omitempty"`
	MaxOutputTokens   *int           `json:"maxOutputTokens,omitempty"`
	Status            *string        `json:"status,omitempty"`
}

// ModelResponse LlmModel 的管理面返回体。
type ModelResponse struct {
	ID                string         `json:"id"`
	OrganizationID    string         `json:"organizationId"`
	ProviderID        string         `json:"providerId"`
	ProviderModelName string         `json:"providerModelName"`
	DisplayName       string         `json:"displayName,omitempty"`
	Capabilities      map[string]any `json:"capabilities,omitempty"`
	MaxContextTokens  *int           `json:"maxContextTokens,omitempty"`
	MaxOutputTokens   *int           `json:"maxOutputTokens,omitempty"`
	Status            string         `json:"status"`
	CreatedAt         string         `json:"createdAt"`
	UpdatedAt         string         `json:"updatedAt"`
}

// ============================================================================
// Alias DTOs
// ============================================================================

// AliasCreate 创建模型别名（alias 本身不直接绑定 modelId，具体路由
// 由 LlmRoute 管理，不在本次 Round 5 范围内）。
type AliasCreate struct {
	OrganizationID string `json:"-"`
	Alias          string `json:"alias"`
	DisplayName    string `json:"displayName,omitempty"`
	Status         string `json:"status,omitempty"`
}

// AliasUpdate 更新模型别名。
type AliasUpdate struct {
	Alias       *string `json:"alias,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// AliasResponse 模型别名返回体。
type AliasResponse struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Alias          string `json:"alias"`
	DisplayName    string `json:"displayName,omitempty"`
	Status         string `json:"status"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// ============================================================================
// API Key DTOs
// ============================================================================

// APIKeyCreate 创建 API Key。GenerateKey() 由 service 层调用，
// plaintext 只会出现在 Create 响应中一次。
type APIKeyCreate struct {
	OrganizationID string   `json:"-"`
	Name           string   `json:"name"`
	AllowedAliases []string `json:"allowedAliases,omitempty"`
	RPMLimit       *int     `json:"rpmLimit,omitempty"`
	TPMLimit       *int     `json:"tpmLimit,omitempty"`
	MonthlyBudget  *float64 `json:"monthlyBudget,omitempty"`
	Currency       string   `json:"currency,omitempty"`
	ExpiresAt      *string  `json:"expiresAt,omitempty"` // RFC3339
}

// APIKeyUpdate 更新 API Key 的限制、名称、允许范围等。
type APIKeyUpdate struct {
	Name           *string  `json:"name,omitempty"`
	AllowedAliases []string `json:"allowedAliases,omitempty"`
	RPMLimit       *int     `json:"rpmLimit,omitempty"`
	TPMLimit       *int     `json:"tpmLimit,omitempty"`
	MonthlyBudget  *float64 `json:"monthlyBudget,omitempty"`
	Currency       *string  `json:"currency,omitempty"`
	ExpiresAt      *string  `json:"expiresAt,omitempty"`
	Status         *string  `json:"status,omitempty"`
}

// APIKeyResponse API Key 返回体。Create 时会额外带 Plaintext 字段，
// 用于让管理员复制后立即丢弃。
type APIKeyResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	KeyPrefix      string   `json:"keyPrefix"`
	Plaintext      string   `json:"plaintext,omitempty"` // 仅 Create 响应带
	AllowedAliases []string `json:"allowedAliases,omitempty"`
	RPMLimit       *int     `json:"rpmLimit,omitempty"`
	TPMLimit       *int     `json:"tpmLimit,omitempty"`
	MonthlyBudget  *float64 `json:"monthlyBudget,omitempty"`
	Currency       string   `json:"currency,omitempty"`
	Status         string   `json:"status"`
	ExpiresAt      *string  `json:"expiresAt,omitempty"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

// ============================================================================
// Route DTOs
// ============================================================================

// RouteCreate 绑定 alias → model 的路由条目。
// Weight + Priority 共同决定 failover 顺序（同 priority 下按 weight 轮询）。
type RouteCreate struct {
	OrganizationID string `json:"-"`
	AliasID        string `json:"aliasId"`
	ModelID        string `json:"modelId"`
	Priority       int    `json:"priority,omitempty"`
	Weight         int    `json:"weight,omitempty"`
	Status         string `json:"status,omitempty"` // active | disabled
}

// RouteUpdate 路由部分更新。所有字段可选。
type RouteUpdate struct {
	ModelID  *string `json:"modelId,omitempty"`
	Priority *int    `json:"priority,omitempty"`
	Weight   *int    `json:"weight,omitempty"`
	Status   *string `json:"status,omitempty"`
}

// RouteResponse 路由返回体，附带 model/provider 名称方便前端展示。
type RouteResponse struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	AliasID        string `json:"aliasId"`
	AliasName      string `json:"aliasName,omitempty"`
	ModelID        string `json:"modelId"`
	ModelName      string `json:"modelName,omitempty"`
	ProviderID     string `json:"providerId,omitempty"`
	ProviderName   string `json:"providerName,omitempty"`
	Priority       int    `json:"priority"`
	Weight         int    `json:"weight"`
	Status         string `json:"status"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// ============================================================================
// Quota DTOs
// ============================================================================

// QuotaCreate 创建一个 RPM/TPM/Cost 限流策略。
// scope 决定配额适用范围：organization / api_key / alias。
// scopeRef 在 scope != organization 时必须提供（具体 api_key id 或 alias id）。
type QuotaCreate struct {
	OrganizationID string  `json:"-"`
	Scope          string  `json:"scope"` // organization | api_key | alias
	ScopeRef       *string `json:"scopeRef,omitempty"`
	LimitType      string  `json:"limitType"` // rpm | tpm | cost
	LimitValue     float64 `json:"limitValue"`
	ResetCycle     string  `json:"resetCycle"` // daily | monthly
	Status         string  `json:"status,omitempty"`
}

// QuotaUpdate 配额策略部分更新。所有字段可选。
type QuotaUpdate struct {
	ScopeRef   *string  `json:"scopeRef,omitempty"`
	LimitType  *string  `json:"limitType,omitempty"`
	LimitValue *float64 `json:"limitValue,omitempty"`
	ResetCycle *string  `json:"resetCycle,omitempty"`
	Status     *string  `json:"status,omitempty"`
}

// QuotaResponse 配额策略返回体。
type QuotaResponse struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	Scope          string  `json:"scope"`
	ScopeRef       *string `json:"scopeRef,omitempty"`
	LimitType      string  `json:"limitType"`
	LimitValue     float64 `json:"limitValue"`
	ResetCycle     string  `json:"resetCycle"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// ============================================================================
// Provider Test Result
// ============================================================================

// ProviderTestResult connectivity test 结果。
// Service 层会尝试向 provider.BaseURL 发送一个最小化的 ping 请求。
type ProviderTestResult struct {
	ProviderID string  `json:"providerId"`
	Reachable  bool    `json:"reachable"`
	LatencyMs  int64   `json:"latencyMs,omitempty"`
	StatusCode *int    `json:"statusCode,omitempty"`
	Error      *string `json:"error,omitempty"`
}

// ============================================================================
// UsageLog DTOs
// ============================================================================

// UsageLogResponse 用量日志返回体（管理面只读）。
type UsageLogResponse struct {
	ID                string  `json:"id"`
	OrganizationID    string  `json:"organizationId"`
	APIKeyID          *string `json:"apiKeyId,omitempty"`
	AliasID           string  `json:"aliasId"`
	ProviderID        string  `json:"providerId"`
	ModelID           string  `json:"modelId"`
	ProtocolFace      string  `json:"protocolFace"`
	AttemptNo         int     `json:"attemptNo"`
	InputTokens       int     `json:"inputTokens"`
	OutputTokens      int     `json:"outputTokens"`
	TotalTokens       int     `json:"totalTokens"`
	CostAmount        float64 `json:"costAmount"`
	Currency          string  `json:"currency"`
	Status            string  `json:"status"`
	ErrorCode         *string `json:"errorCode,omitempty"`
	TTFTMs            *int    `json:"ttftMs,omitempty"`
	TotalLatencyMs    *int    `json:"totalLatencyMs,omitempty"`
	UpstreamRequestID *string `json:"upstreamRequestId,omitempty"`
	FailoverFrom      *string `json:"failoverFrom,omitempty"`
	RequestID         string  `json:"requestId"`
	CreatedAt         string  `json:"createdAt"`
}

// =============================================================================
// Price Version DTOs
// =============================================================================

// PriceVersionCreate creates a new price snapshot for a model.
// version_no is auto-incremented by the service; callers don't supply it.
type PriceVersionCreate struct {
	OrganizationID   string  `json:"-"`
	ModelID          string  `json:"modelId"`
	InputPricePer1M  float64 `json:"inputPricePer1M"`
	OutputPricePer1M float64 `json:"outputPricePer1M"`
	Currency         string  `json:"currency,omitempty"`
	EffectiveFrom    string  `json:"effectiveFrom"` // RFC3339
}

// PriceVersionUpdate updates a price snapshot. Only the financial fields are mutable;
// modelId and version_no are structural and locked after creation.
type PriceVersionUpdate struct {
	InputPricePer1M  *float64 `json:"inputPricePer1M,omitempty"`
	OutputPricePer1M *float64 `json:"outputPricePer1M,omitempty"`
	Currency         *string  `json:"currency,omitempty"`
	EffectiveFrom    *string  `json:"effectiveFrom,omitempty"`
}

// PriceVersionResponse is the management-facing return body.
type PriceVersionResponse struct {
	ID               string  `json:"id"`
	OrganizationID   string  `json:"organizationId"`
	ModelID          string  `json:"modelId"`
	VersionNo        int     `json:"versionNo"`
	InputPricePer1M  float64 `json:"inputPricePer1M"`
	OutputPricePer1M float64 `json:"outputPricePer1M"`
	Currency         string  `json:"currency"`
	EffectiveFrom    string  `json:"effectiveFrom"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

// =============================================================================
// Provider Health Response (read-only)
// =============================================================================

// ProviderHealthResponse is the management-facing view of ProviderHealth.
// State machine transitions happen inside GatewayService / HealthProbeWorker;
// the admin API is read-only.
type ProviderHealthResponse struct {
	ID                  string  `json:"id"`
	ProviderID          string  `json:"providerId"`
	State               string  `json:"state"`
	ConsecutiveFailures int     `json:"consecutiveFailures"`
	OpenUntil           *string `json:"openUntil,omitempty"`
	LastCheckedAt       *string `json:"lastCheckedAt,omitempty"`
	LastLatencyMs       *int    `json:"lastLatencyMs,omitempty"`
	UpdatedAt           string  `json:"updatedAt"`
}
