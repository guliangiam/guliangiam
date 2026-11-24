package aigateway

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// metricNamespace 与 middleware/prometheus.go 保持一致（naming.yaml N14）。
const metricNamespace = "guliangiam"

var (
	// gatewayCallsTotal 记录每次 upstream 调用（成功或失败）。
	// Labels: organization, provider, model, alias, protocol_face, status, call_type
	gatewayCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "aigateway_calls_total",
			Help:      "Total number of AI Gateway upstream calls (chat / chat_stream / embedding)",
		},
		[]string{metaOrganization, metaProvider, fieldModel, metaAlias, metaProtocolFace, metaStatus, metaCallType},
	)

	// gatewayTokensTotal 记录 token 用量（成功调用）。
	// Labels: organization, provider, model, alias, protocol_face, token_type (input/output)
	gatewayTokensTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "aigateway_tokens_total",
			Help:      "Total input/output tokens consumed through the AI Gateway",
		},
		[]string{metaOrganization, metaProvider, fieldModel, metaAlias, metaProtocolFace, "token_type"},
	)

	// gatewayCostTotal 记录上游调用成本（元，成功调用）。
	// Labels: organization, provider, model, alias, protocol_face
	gatewayCostTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "aigateway_cost_total",
			Help:      "Total cost (yuan) of upstream AI calls",
		},
		[]string{metaOrganization, metaProvider, fieldModel, metaAlias, metaProtocolFace},
	)

	// gatewayUpstreamLatency 记录上游调用延迟（秒，成功+失败都记录）。
	// Buckets 10ms → 60s 覆盖本地模型（<100ms）→ OpenAI turbo（<2s）→ 长模型（<30s）。
	// Labels: organization, provider, model, alias, protocol_face, status
	gatewayUpstreamLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "aigateway_upstream_latency_seconds",
			Help:      "Latency of upstream AI provider calls (seconds)",
			Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{metaOrganization, metaProvider, fieldModel, metaAlias, metaProtocolFace, metaStatus},
	)

	// gatewayQuotaExceededTotal 记录配额超限次数（enforceQuota 拒绝）。
	// Labels: organization, api_key, quota_scope, limit_type (rpm/tpm/cost)
	gatewayQuotaExceededTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "aigateway_quota_exceeded_total",
			Help:      "Total number of quota-limit rejections (RPM / TPM / cost)",
		},
		[]string{metaOrganization, "api_key", metaQuotaScope, "limit_type"},
	)

	// gatewayToolExecutionsTotal 记录工具调用执行次数（成功或失败）。
	// Labels: organization, provider, tool_name, status
	gatewayToolExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "aigateway_tool_executions_total",
			Help:      "Total number of tool_use blocks executed by the gateway (MCP-backed)",
		},
		[]string{metaOrganization, metaProvider, "tool_name", metaStatus},
	)

	// gatewayCircuitState 记录 provider 健康状态（Gauge）。
	// Prometheus 状态码映射：0=healthy, 1=degraded, 2=open, 3=half_open
	gatewayCircuitState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "aigateway_provider_circuit_state",
			Help:      "Current circuit-breaker state per provider (0=healthy, 1=degraded, 2=open, 3=half_open)",
		},
		[]string{metaOrganization, metaProvider},
	)
)

// Prometheus 状态值（用于 gatewayCircuitState Gauge）。
const (
	circuitStateHealthy  float64 = 0
	circuitStateDegraded float64 = 1
	circuitStateOpen     float64 = 2
	circuitStateHalfOpen float64 = 3
)

// ─── 类型化 labels 避免各处手动拼 []string ──────────────────────────

// gatewayLabels 打包从 route resolution + api key 派生的 common label values。
// 用 struct 是为了避免 gateway_service.go / ChatStream tee goroutine / tool_service.go
// 各处重复拼 []string{"org", "provider", ...} 并可能漏字段。
type gatewayLabels struct {
	organization string
	provider     string
	model        string
	alias        string
	protocolFace string
	callType     string // "chat" / "embedding" / "stream"
}

func buildGatewayLabels(key *domain.LlmApiKey, res *domain.RouteResolution, callType string) gatewayLabels {
	l := gatewayLabels{callType: callType}
	if key != nil {
		l.organization = key.OrganizationID
	}
	if res != nil {
		if res.Provider != nil {
			l.provider = res.Provider.ID
		}
		if res.Model != nil {
			l.model = res.Model.ID
		}
		l.protocolFace = res.ProtocolFace
		if res.Route != nil {
			l.alias = res.Route.AliasID
		}
	}
	return l
}

// circuitStateToFloat 将 ProviderHealth.State 字符串映射为 Prometheus 数值。
func circuitStateToFloat(state string) float64 {
	switch state {
	case domain.HealthStateOpen:
		return circuitStateOpen
	case domain.HealthStateDegraded:
		return circuitStateDegraded
	case domain.HealthStateHalfOpen:
		return circuitStateHalfOpen
	default:
		return circuitStateHealthy
	}
}

// ─── 记录函数 ────────────────────────────────────────────────────────

// RecordGatewayCall 记录 upstream 调用计数（成功 +1 或失败 +1）。
func RecordGatewayCall(l gatewayLabels, status string) {
	gatewayCallsTotal.WithLabelValues(
		l.organization, l.provider, l.model, l.alias, l.protocolFace, status, l.callType,
	).Inc()
}

// RecordGatewayLatency 记录 upstream 调用延迟（秒）。
// status 必须与 RecordGatewayCall 传入的值匹配（"success" / "failure"）——
// 两个指标共享同一组 label values 便于 Grafana 关联查询
// `rate(guliangiam_aigateway_calls_total[5m]) by (status)` vs
// `histogram_quantile(0.95, rate(guliangiam_aigateway_upstream_latency_seconds_bucket[5m])) by (status)`。
func RecordGatewayLatency(l gatewayLabels, status string, latencySec float64) {
	gatewayUpstreamLatency.WithLabelValues(
		l.organization, l.provider, l.model, l.alias, l.protocolFace, status,
	).Observe(latencySec)
}

// RecordGatewayTokens 记录 token 用量（成功调用时）。
func RecordGatewayTokens(key *domain.LlmApiKey, res *domain.RouteResolution, callType string, resp *domain.CanonicalResponse) {
	if resp == nil {
		return
	}
	l := buildGatewayLabels(key, res, callType)
	if resp.InputTokens > 0 {
		gatewayTokensTotal.WithLabelValues(
			l.organization, l.provider, l.model, l.alias, l.protocolFace, "input",
		).Add(float64(resp.InputTokens))
	}
	if resp.OutputTokens > 0 {
		gatewayTokensTotal.WithLabelValues(
			l.organization, l.provider, l.model, l.alias, l.protocolFace, "output",
		).Add(float64(resp.OutputTokens))
	}
}

// RecordGatewayCost 记录上游调用成本（元）。
// 需要从 PriceVersionsRepo 查询价格。价格查询失败 → 不记录成本（失败 best-effort）。
// ctx 从调用方（execute / ChatStream tee goroutine）传入——repo 实现可能需要它做
// tracing 透传或超时控制；绝不能传 nil（Advisor ADR-038）。
func RecordGatewayCost(ctx context.Context, key *domain.LlmApiKey, res *domain.RouteResolution, callType string, resp *domain.CanonicalResponse, prices domain.PriceVersionsRepo) {
	if resp == nil || res == nil || res.Model == nil || prices == nil {
		return
	}
	l := buildGatewayLabels(key, res, callType)
	pv, err := prices.GetByModel(ctx, res.Model.ID)
	if err != nil || pv == nil {
		return
	}
	cost := pv.CalculateCost(resp.InputTokens, resp.OutputTokens)
	if cost > 0 {
		gatewayCostTotal.WithLabelValues(
			l.organization, l.provider, l.model, l.alias, l.protocolFace,
		).Add(cost)
	}
}

// RecordQuotaExceeded 记录配额超限事件（enforceQuota 拒绝时调用）。
// scope: "api_key" / "quota_policy"; limitType: "rpm" / "tpm" / "cost"
func RecordQuotaExceeded(orgID, apiKeyID, scope, limitType string) {
	gatewayQuotaExceededTotal.WithLabelValues(orgID, apiKeyID, scope, limitType).Inc()
}

// RecordToolExecution 记录工具调用执行（成功或失败）。
// toolName: MCP tool name; status: "success" / "failure"
func RecordToolExecution(orgID, providerID, toolName, status string) {
	gatewayToolExecutionsTotal.WithLabelValues(orgID, providerID, toolName, status).Inc()
}

// RecordCircuitState 更新 provider circuit state Gauge。
// 每次健康状态变化（成功恢复 / 失败触发熔断 / HealthProbeWorker 探测后）调用。
func RecordCircuitState(orgID, providerID, state string) {
	gatewayCircuitState.WithLabelValues(orgID, providerID).Set(circuitStateToFloat(state))
}
