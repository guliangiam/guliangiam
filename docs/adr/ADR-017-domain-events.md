# ADR-017: 领域事件与事件总线

- 状态：[Current]/[Target] — 事件注册表 + audit_outbox 同事务写入 + Worker 投递链路已落地；投递不丢不重演练与事件 schema 兼容规则取证为 [Target]（出口条件完成后转 [Current]）
- 日期：2026-09-14

## 背景

当前副作用（审计、通知、Webhook、身份同步）散落在各用例内直接调用：写库后同步调用外部通道，调用失败要么吞掉（审计不完整）要么拖垮主流程；模块之间因此硬耦合，新增一个消费者就要改用例。审计与集成需要一种“事实变更即可被可靠观察”的机制，同时又不能走向完整事件溯源、动摇 PostgreSQL 的事实源地位（ADR-002）。

## 决策

领域聚合在状态变更时发布版本化领域事件（如 `user.created`、`session.revoked`、`token.reused`、`policy.changed`、`key.rotated`）；事件在业务事务内写入 Outbox（ADR-008），由 Worker 投递给审计、通知、同步与 Webhook 消费者；事件携带类型与版本号，消费者必须幂等（ADR-018）；事件仅用于通知与集成，**不做事件溯源**，不替代当前状态存储。

## 理由与后果

- 正面：审计链完整（事实写入即事件必达，见 ADR-008）；新增消费者订阅事件即可，不再改用例；消费者失败可重试/死信而不污染主事务；事件版本化为后续演进留出兼容窗口。
- 负面：需要维护事件 schema 注册表与版本兼容策略（加字段不破坏、改语义需新版本）；消费者幂等成为强制纪律；最终一致性让“写完立刻读到外部已响应”的时序假设失效，需要在产品交互上考虑；事件量级受 outbox 单表承载约束（ADR-011）。

## 替代方案

- **维持用例内同步直调（不采用）**：崩溃窗口丢副作用、外部故障级联进主请求、模块硬耦合；已退役的 webhook 直轮询 Worker（H40 前直接轮询 `webhook_events` 自建投递/重试）证明了投递逻辑被迫自建。
- **事件溯源（事件日志为唯一事实、状态由回放派生）**：明确放弃——与 ADR-002（PostgreSQL 当前状态为事实源）根本冲突，回放/快照/模式演进复杂度远超当前需要；本 ADR 的事件是“状态变更的通知”，状态表仍是权威。
- **直接订阅 PostgreSQL WAL（CDC）对外分发**：放弃理由——WAL 是行变更而非领域语义，消费者要反推业务事件；保留为超大规模时 ADR-008 的投递介质复审项，不作为事件定义层。

## 验证证据

**实现证据（`[Current]` 子事实）**：

- 承载表已建：`guliangiam.outbox`（含 `aggregate_type/aggregate_id/event_type/event_version/payload/headers`，结构即按领域事件设计；见 ADR-008）。
- 事件模型已落地：事件类型注册表在 `contracts/registries/events.yaml`（生成渲染层 `docs/generated/event-registry.md`）；业务应用域经 `audit_outbox.go` 在事务内写 outbox（`grep -rn 'audit_outbox' internal --include='*.go'` 命中多个域），relay/Worker 由 `internal/bootstrap/wire_outbox.go` 装配，领域事件经 `DomainEventOutboxHandler` 扇出到租户 webhook 订阅者。
- webhook 投递已收敛为 outbox 单轨（H40）：遗留 `webhook_events` 直轮询 Worker 已删除，`DeliveryOutboxHandler`（`internal/app/webhooks/delivery_outbox.go`）统一投递并维护台账（口径与 ADR-008/ADR-018 一致）。审计在用例内直调（如 `internal/app/records/create_record.go`、`internal/app/sessions/service.go` 引用 audit）。

**出口条件（[Target] 待取证项）**：事件类型与版本注册表、事务内写 outbox、Worker 投递均已落地（见上）；webhook 遗留直轮询轨已收敛到 outbox 单轨（H40）；剩余：事件 schema 兼容规则文档化；完成投递不丢不重演练后转 [Current]。

## 复审条件

出现跨服务异步编排或需要事件重放重建读模型（CQRS/事件溯源）的真实需求时，另开 ADR 重新评估事实模型；事件量级超出 PG outbox 经济区间时联动 ADR-008/ADR-011 复审介质与分区。
