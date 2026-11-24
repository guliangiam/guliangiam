package aigateway

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
)

// StubPolicy 是 domain.TopologyPolicy 的合约层存根——声明 embedded 模式 + ADR-034 定义的三条触发阈值。
// 真实部署时由 infra adapter 替换为 Prometheus 查询 + 进程拓扑探测。
type StubPolicy struct{}

func NewStubPolicy() *StubPolicy { return &StubPolicy{} }

func (s *StubPolicy) Current() domain.DeploymentMode { return domain.DeploymentModeEmbedded }

func (s *StubPolicy) Triggers() []domain.TopologyTrigger {
	return []domain.TopologyTrigger{
		{
			Kind:        domain.TriggerSLODegradation,
			Threshold:   "P99 > 500ms OR SLO 99.9% 跌破",
			Description: "AI Gateway 流式请求长时间占用 goroutine/连接池，污染认证面性能",
		},
		{
			Kind:        domain.TriggerThroughputThreshold,
			Threshold:   "非流式 QPS > 5000 OR 流式并发 > 500",
			Description: "当前 IAM 进程目标是非流式数千 QPS，AI 网关峰值超出后应独立",
		},
		{
			Kind:        domain.TriggerDataResidency,
			Threshold:   "客户要求数据面部署在 VPC 内，控制面托管",
			Description: "物理隔离部署形态需求",
		},
	}
}

func (s *StubPolicy) Evaluate() (bool, domain.TopologyTriggerKind, error) {
	// 存根：无监控数据查询能力，始终返回未触发。
	// 真实 adapter 将查 Prometheus guliangiam_http_requests_p99 / ai_gateway_stream_concurrent 等指标。
	return false, "", nil
}
