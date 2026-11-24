# ADR-008: Outbox 与幂等 Worker

- 状态：[Current]/[Target] — outbox 表 + 事务内写入 + Worker SKIP LOCKED 投递链路已落地；杀进程/重复投递/死信恢复故障演练为 [Target]（出口条件完成后转 [Current]）
- 日期：2026-09-14

## 背景

业务事务与外部副作用（Webhook、通知、身份同步、支付回调）无法处于同一事务：先调外部再写库，外部成功而本地回滚会产生“幽灵副作用”；先写库再调外部，进程在两步之间崩溃就会永久丢事件；失败重试又要求副作用可识别重复。随着 Webhook/同步消费者增多，必须在项目早期定下事务性投递范式，而不是让每个用例各自直连。

## 决策

业务事务只写业务事实和 Outbox 行（同一事务）；外部通知、Webhook、同步和支付副作用由独立 Worker 基于租约取件并投递，具备幂等键、指数退避、最大重试次数、死信状态与人工重放能力；消费者侧幂等由 ADR-018 统一约束。

## 理由与后果

- 正面：事实写入与事件投递原子化（至少一次投递）；崩溃后可凭 outbox 重放，事件链可审计；各用例不再散落直连外部系统，与 ADR-017 的领域事件模型自然衔接。
- 负面：引入投递延迟（轮询/租约粒度）与一张高写入表的容量治理（ADR-011）；Worker 租约、重试退避、死信运营与重放工具都需要建设和演练；消费者必须幂等，否则“至少一次”会变成重复副作用。

## 替代方案

- **用例内同步直调外部 API**：放弃理由——请求时延与可用性被外部系统绑定（故障级联），本地事务与外部调用之间的崩溃窗口会丢事件，且无法统一重试/死信。
- **事务提交后立即发消息队列（非事务消息）**：放弃理由——“本地提交成功、发信失败”的窗口依旧存在，消息队列还会变成第二个事实源；只有事务性 Outbox 能保证事实与事件同生共死。
- **基于逻辑解码的 CDC 捕获变更**：放弃理由——运维与运维心智成本高，且变更日志不直接承载事件语义（聚合事件需要业务回填）；保留为未来事件量级超出 PG outbox 承载时的复审选项。

## 验证证据

**现状事实（`[Current]` 子事实）**：

- `db/postgresql/V1__guliangiam_schema_init.sql` 已建 `guliangiam.outbox`：含 `status` 状态机（`pending/processing/succeeded/failed/dead` 的 CHECK）、`attempts/max_attempts`（默认 12）、`locked_by/locked_at` 租约列、`available_at` 与 `WHERE status = 'pending'` 部分索引。

**实现证据**：

- 业务写路径必须在事务内写 Outbox：每个业务应用域提供 `audit_outbox.go`，嵌入审计记录与事件（复现命令 `grep -rn 'audit_outbox' internal --include='*.go'`）。
- webhook 投递已收敛为 **outbox 单轨**（H40）：`outbox`（`aggregate_type='webhook'`）由 `platform/outbox.Worker` 投递，处理器为 `internal/app/webhooks/delivery_outbox.go`（DeliveryOutboxHandler，投递同时维护 `webhook_events` 台账与 `webhook_event.failed` 审计），HMAC 签名在 `internal/infra/webhook/http_sender.go`；领域事件经 `DomainEventOutboxHandler` 扇出到租户 webhook 订阅者；重放（`ReplayWebhookEvent`）同事务重置台账并入队新 outbox 行。遗留 `webhook_events` 直轮询 Worker 已删除（历史见 CHANGELOG）。

**目标缺口（`[Target]` 待取证项）**：

- Worker 租约取件、退避重试、死信与重放的故障演练（杀进程、重复投递、死信恢复）尚未完成。
- 完成故障演练后，本 ADR 转为 [Current]。

**出口条件**：业务写路径在事务内写 outbox（grep 能命中仓储且有单测证明同事务）；Worker 具备租约/退避/最大重试/死信/人工重放与指标；完成杀进程、重复投递、死信恢复的故障演练后，本 ADR 转为 [Current]。

## 复审条件

ADR-017 领域事件落地时与本 ADR 联动评审；事件量级超过 PG outbox 轮询经济区间时评估 CDC 或事务消息；公司级消息中间件定型后重评投递介质，但“业务事务内落事件”的约束不得放宽。
