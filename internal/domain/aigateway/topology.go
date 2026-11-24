package aigateway

// DeploymentMode declares where the AI Gateway data plane runs.
// Canonical 核心与进程位置解耦——独立部署只需搬运 infra/aiupstream + infra/aicache。
type DeploymentMode int

const (
	// DeploymentModeEmbedded — 数据面与 GuliangIAM 主进程同一二进制（首版默认）。
	DeploymentModeEmbedded DeploymentMode = iota
	// DeploymentModeSidecar — 数据面作为 sidecar 同 Pod 独立进程。
	DeploymentModeSidecar
	// DeploymentModeStandalone — 数据面完全独立部署（VPC 数据驻留合规场景）。
	DeploymentModeStandalone
)

func (m DeploymentMode) String() string {
	switch m {
	case DeploymentModeEmbedded:
		return "embedded"
	case DeploymentModeSidecar:
		return "sidecar"
	case DeploymentModeStandalone:
		return "standalone"
	default:
		return "unknown"
	}
}

// TopologyTrigger 是触发独立部署评估的三条条件之一。
// 任何一条命中都必须触发 split evaluation（ADR-034 决策）。
type TopologyTrigger struct {
	Kind        TopologyTriggerKind
	Threshold   string
	Description string
}

// TopologyTriggerKind 枚举三条触发条件类型。
type TopologyTriggerKind string

const (
	TriggerSLODegradation      TopologyTriggerKind = "slo_degradation"
	TriggerThroughputThreshold TopologyTriggerKind = "throughput_threshold"
	TriggerDataResidency       TopologyTriggerKind = "data_residency"
)

// TopologyPolicy 是部署拓扑的 domain port。
// infra adapter 可探测当前进程拓扑并返回触发条件声明；
// stub adapter 返回 embedded + 默认阈值声明（首版目标态）。
type TopologyPolicy interface {
	// Current 返回当前部署模式。
	Current() DeploymentMode
	// Triggers 返回三条触发条件的冻结声明（用于运维监控面板 / 发布门禁）。
	Triggers() []TopologyTrigger
	// Evaluate 返回当前是否命中任何触发条件。
	// 真实 adapter 会查 Prometheus/监控数据；stub 始终返回 false。
	Evaluate() (bool, TopologyTriggerKind, error)
}
