# ADR-034: AI 网关部署形态（同进程 vs 独立数据面）

- 状态：[Current]/[Target]（TopologyPolicy port + Stub adapter 已落地——DeploymentMode + 三条触发条件冻结声明 + Evaluate 探测；端到端 SLO 监控与 Canonical 解耦搬移测试仍为 [Target]）
- 日期：2026-10-14

## 背景

AI 网关设计需裁决"数据面与控制面是否同进程"。GuliangIAM 主进程承载认证、IAM、商业化等多域，AI 网关数据面（流式 SSE + 上游 LLM 调用）可能占用大量 goroutine/连接池。同进程可复用全部治理能力（认证、审计、RLS、限流、可观测性），独立部署可避免流量污染但重复建设。

## 决策

**首版同进程实现**——AI 网关数据面与 GuliangIAM 主进程同一二进制。预留独立数据面扩展点，以下**三条触发条件任一命中**时必须评估拆分：

| 触发信号 | 阈值 | 说明 |
|---------|------|------|
| **SLO 降级** | GuliangIAM 认证面 P99 > 500ms 或 SLO 99.9% 跌破 | AI 网关流式请求长时间占用 goroutine/连接池，污染认证面性能 |
| **吞吐阈值** | AI Gateway 非流式 QPS > 5000 或流式并发 > 500 | 当前 IAM 进程目标是非流式数千 QPS，AI 网关峰值超出后应独立 |
| **数据驻留合规** | 客户要求数据面部署在 VPC 内，控制面托管 | 物理隔离部署形态需求 |

## 替代方案

- **一开始就独立部署**：丢治理能力复用收益，增加初期运维成本（独立监控/日志/熔断/限流）；否决——AI 网关初期流量可控。
- **数据面 + 控制面完全分离 + 配置同步 API**：架构准备已做（Canonical 核心解耦；配置缓存为规划态，见下节），拆分工作量预估 2-3 人周；作为触发后的行动方案保留。

## 架构准备（支持未来独立；已建成项与规划项分别标注）

1. Canonical 核心与协议面/上游适配器解耦（App 层只依赖 Domain Port）
2. 配置缓存 TTL 30s + 写路径主动失效机制（**规划态 [Target]**：`internal/infra/aicache/config_cache.go`（待建）——`ls internal/infra/` 无 `aicache/`，`grep -rln ConfigCache internal/ --include='*.go'` 零命中，核验 2026-11-21；`internal/domain/aigateway/topology.go` 注释中的 aicache 引用同属规划树）
3. 数据面表（`llm_*`）完全独立于 IAM 业务表
4. SSRF / 熔断 / 配额治理复用 `platform/` 组件（数据面可独立引用）
5. Domain 层无对 IAM 业务域的硬依赖（仅通过 `organization_id` + RLS 关联租户）

## 理由与后果

首版同进程实现是务实决策：AI 网关作为全新建设域，初期流量可控，内嵌后零成本复用 GuliangIAM 已验证的治理体系（RLS、Casbin、Outbox、审计、可观测性）。Canonical 核心设计已与进程位置解耦，独立数据面只需搬运 `infra/aiupstream/` 并补齐规划态的 `infra/aicache/`（尚未建成，见上节）与配置同步端点，核心代码零改动。

代价：需持续监控三条触发条件指标，一旦命中必须快速评估拆分。

## 验证证据

- Current：`internal/platform/ssrf/`、`internal/platform/errors/`、`internal/domain/outbox/`、`bootstrap/telemetry.go` 均可被 aigateway 复用，无硬依赖。
- Target 出口条件：①AI 网关端到端成功且 SLO 指标未达触发阈值（P0-P2）；②三条触发条件指标冻结在门禁监控面板（`registries/slo.yaml`）；③Canonical 核心解耦测试通过（搬移 aiupstream 到独立进程后单元测试零改动；前置依赖：规划态 `aicache/` 配置缓存先建成）。

## 复审条件

三条触发条件任一命中，或 Canonical 核心需要独立序列化存储时，复审本 ADR。
