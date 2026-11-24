# ADR-020: SLO 与错误预算

- 状态：[Current]/[Target]（分面 SLI/SLO 注册表、错误预算评估工具、多窗口燃尽率告警与 CI/Make 发布门禁已落地；分面看板与 L1/L2 连续测量仍为 [Target]）
- 日期：2026-09-14

## 背景

系统当前已有 Prometheus 指标与按阈值触发的告警，但阈值回答的是“现在是否异常”，回答不了“这个月服务质量是否达标、还剩多少犯错空间、是否值得为了可靠性停止发布”。没有分面 SLO 时，管理后台与登录/协议端点被笼统地用同一个数字概括，而二者的消费者与风险完全不同（STD 4.1.3 三分面）。必须先把 SLI/SLO 口径、错误预算与采样策略定下来，指标采集才能围绕目标组织。

## 决策

为关键路径定义 SLI/SLO 与错误预算，并据此设置多窗口燃尽率告警与发布门禁。可用性 SLO 与 API 三分面口径一致，**具体数值与唯一权威表位于《开发规范手册》11.7**（本 ADR 不复写数值，避免与唯一源分岔）；口径要点：按面分别定义月度可用性目标，不得用单一全局数字概括全部接口。

分布式追踪采用自适应采样：错误与敏感操作（登录、令牌、授权、密钥相关）全采样，普通请求按基础采样率；指标标签必须保持低基数（禁止把用户 ID、请求 ID 放进标签）。SLO 不得用于掩盖单次严重安全事故——安全事件走独立的应急与披露流程，不用“预算未烧完”解释。

## 理由与后果

- 正面：可靠性目标可量化，发布节奏与错误预算绑定（预算烧尽即冻结非安全发布）；三分面避免用管理面数字稀释认证/协议面的要求；自适应采样在成本可控下保留故障全样本。
- 负面：需要随 STD 4.1.3 落地 `surface="management|authentication|protocol"` 指标标签与路由分面；要长期维护 SLI 定义、燃尽率告警与低基数纪律；SLO 目标需要用 L1/L2（真实数据库/真实 HTTP）测量支撑，L0 纯函数基准不得用于宣称达标；目标定得过高会造成告警疲劳，需要按连续月份的实际数据校准。

## 替代方案

- **沿用单一全局 99.9% 不分面**：放弃理由——STD 指标表明确规定“不得再用笼统的单一 99.9% 概括全部接口”（STD §12 可用性口径注记，2026-09-14 只读核对）：管理面与认证/协议面消费者、限流组、错误格式都不同，一个数字无法驱动差异化门禁。
- **只保留告警阈值、不设 SLO/错误预算（现状固化）**：放弃理由作为目标态——阈值没有时间窗目标就没有“剩余预算”概念，无法回答是否应停发；告警数量与服务质量脱钩。
- **对所有请求 100% 全采样追踪**：放弃理由——成本与存储量不可接受，且普通成功请求对故障诊断价值低；错误/敏感全采样 + 基础率采样已覆盖诊断需要。

## 验证证据

**实现证据（`[Current]` 子事实）**：

- Prometheus 指标链路与分面标签已存在：路由三面分组与 `surface` 指标维度（`surface="management|authentication|protocol"`）已落地，`internal/bootstrap/server.go` 注册 `/metrics`，`internal/infra/postgres/shared/query_monitor.go` 提供查询侧指标。
- SLO 数值的唯一权威表位于《开发规范手册》11.7，与本 ADR 的分面口径一致。
- 错误预算评估工具已落地：`cmd/slo-budget` 读取 `contracts/registries/slo.yaml`，从 Prometheus 查询 28 天 `increase()` 分子/分母并调用 `internal/platform/slo.Evaluate` 计算每个面的剩余预算百分比；`-min-remaining`（默认 10）以下退出码 1，可直接接入发布流水线。预算算术（allowed/consumed/remaining）与 PromQL 构造有单元测试。
- 告警与门禁接线：`deploy/alertmanager-rules.yml` 的 `guliangiam_slo_budget` 组提供 5m/1h/28d 错误率记录规则与 14.4x 多窗口燃尽率告警（99.9% 认证/协议面 critical、99.5% 管理面 warning）；`make slo-budget SLO_PROMETHEUS_URL=…` 调用评估工具，CI `slo_gate` job 在仓库变量 `SLO_PROMETHEUS_URL` 配置时执行同一门禁。

**目标缺口（`[Target]` 待取证项）**：

- 分面 SLI 看板、L1/L2 连续测量与自适应采样器仍为 [Target]；`/metrics` 的抓取面加固（独立端口/鉴权/ServiceMonitor）见《部署与运维指南》5。

**出口条件**：

1. **分面 SLI 看板上线**：Grafana/看板系统有三个独立面板（`surface=management|authentication|protocol`），每个面板展示当前 28 天滚动成功率、剩余预算分钟数、多窗口燃尽率（1h/6h/24h）；看板数据源为 Prometheus `guliangiam_http_requests_total` + `guliangiam_http_request_errors_total`（按 `surface` 标签聚合）。验证：`curl -s http://<prom>/api/v1/query?query=guliangiam_http_requests_total{surface="authentication"}` 返回非空 series。
2. **燃尽率告警演练通过**：手工注入 5xx 故障（`curl -X POST /api/v1/__debug__/inject_fault?code=503`），在 10 分钟内触发「快速燃烧」告警（预算耗尽速度 > 14.4x 基准），on-call 人员在 5 分钟内收到通知（IM/电话）；演练记录归档至《部署与运维指南》。
3. **发布门禁与预算联动**：评估工具已落地（`go run ./cmd/slo-budget -prometheus <url> -min-remaining 10`）；剩余预算 < 10% 时退出码 1，供 CI 阻塞非安全修复类发布（安全修复 `fix(security):` 由流水线豁免）。CI `slo_gate` 与 `deploy/alertmanager-rules.yml` 的多窗口燃尽率告警已接线（见上节实现证据）；分面 SLI 看板仍为 [Target]，登记于 `contracts/registries/open-items.yaml`（出口条件以该注册表 `close_condition` 为准）。
4. **L1/L2 测量连续产出一个月**：L1 测量（真实 HTTP + 真实 PostgreSQL）连续 30 天每日执行并产出 SLI 快照，L2 测量（真实 HTTP + mock 依赖）作为日常流水线执行；任一 SLI 低于 SLO 目标（management 99.5% / authentication+protocol 99.9%）触发告警并归因。测量脚本与每日快照文件尚未落地，作为开放项登记于 `contracts/registries/open-items.yaml`。

## 复审条件

引入多区域部署（SLO 需要跨区合成口径）或 SLO 目标需按租户/套餐差异化时复审；连续多月预算大量剩余或频繁烧尽时校准目标值与告警阈值；STD 三分面口径若修订，本 ADR 数值必须同步并在此记录变更。
